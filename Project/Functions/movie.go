package Functions

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"html/template"
	"mime"
)

// VideoTemplateDataは動画表示テンプレートに渡すデータを定義します。
type VideoTemplateData struct {
	WS_Title string
	WS_Path  string
	WS_BaseURL template.URL
}

// HandleMoviePageは動画再生ページをレンダリングします。
func HandleMoviePage(resolvedFolders map[string]string, config *ServerConfig, movieTmpl *template.Template, err404Tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestedPath := getRequestedPath(r)
		
		// .movie.htmlを取り除いて元の動画ファイルパスを取得
		originalPath := strings.TrimSuffix(requestedPath, ".movie.html")

		// URLエンコードされた元のファイル名を取得
		originalFileName := filepath.Base(originalPath)

		// リンク生成のために元のフォルダパスを取得
		parentURL := filepath.Dir(r.URL.Path)
		if parentURL == "." {
			parentURL = ""
		}
		parentURL = "/" + url.PathEscape(parentURL) + "/"

		// テンプレートに渡すデータを作成
		imageData := VideoTemplateData{
			WS_Title:   originalFileName,
			WS_Path:    "/" + originalPath,
			WS_BaseURL: template.URL(parentURL),
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := movieTmpl.Execute(w, imageData); err != nil {
			log.Printf("テンプレートの実行に失敗しました: %v", err)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusInternalServerError)
			err404Tmpl.Execute(w, NotFoundData{WS_Path: r.URL.Path})
		}
	}
}

// HandleMovieStreamingは動画ファイルをFFmpegでMP4に変換してストリーミングします。
func HandleMovieStreaming(resolvedFolders map[string]string, config *ServerConfig, err404Tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestedPath := getRequestedPath(r)

		var fullPath string
		pathParts := strings.Split(requestedPath, string(os.PathSeparator))
		firstFolder := pathParts[0]

		if resolvedPath, ok := resolvedFolders[firstFolder]; ok {
			fullPath = resolvedPath
			if len(pathParts) > 1 {
				subPath := filepath.Join(pathParts[1:]...)
				fullPath = filepath.Join(resolvedPath, subPath)
			}
			
			info, err := os.Stat(fullPath)
			if err != nil || !info.Mode().IsRegular() || !IsMovieFile(fullPath) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusNotFound)
				err404Tmpl.Execute(w, NotFoundData{WS_Path: r.URL.Path})
				return
			}
			
			// FFmpegコマンドの実行
			cmd := exec.Command("ffmpeg", "-i", fullPath, "-c:v", "copy", "-c:a", "aac", "-f", "mp4", "-movflags", "frag_keyframe+empty_moov", "-")
			
			cmd.Stdout = w
			cmd.Stderr = os.Stderr

			w.Header().Set("Content-Type", "video/mp4")
			w.Header().Set("Accept-Ranges", "bytes")

			if err := cmd.Run(); err != nil {
				log.Printf("FFmpegの実行に失敗しました: %v", err)
				return
			}
		} else {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			err404Tmpl.Execute(w, NotFoundData{WS_Path: r.URL.Path})
		}
	}
}

// IsMovieFileはファイルが動画ファイルであるかチェックします。
func IsMovieFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	// 広く使われている動画拡張子をチェック
	return ext == ".mkv" || ext == ".mov" || ext == ".avi" || ext == ".webm" || ext == ".mp4" || ext == ".wmv" || ext == ".flv"
}

// getRequestedPathはリクエストされたパスを正規化し、セキュリティ上の問題を回避します。
func getRequestedPath(r *http.Request) string {
	path := strings.TrimPrefix(r.URL.Path, "/")
	path, _ = url.PathUnescape(path) // URLデコードを行う
	path = filepath.Clean(path)
	if path == "." {
		return ""
	}
	return path
}
