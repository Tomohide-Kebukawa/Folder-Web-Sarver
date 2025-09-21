// Functions/movie.go:動画プレイヤーハンドラ:Functions/movie.go

package internal

import (
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// HandleMoviePageは動画再生ページをレンダリングします。
func HandleMoviePage(resolvedFolders map[string]string, config *ServerConfig, movieTmpl *template.Template, err404Tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestedPath := getRequestedPath(r)

		// 元の動画ファイルパスを取得するために.movie.htmlを削除
		originalPath := strings.TrimSuffix(requestedPath, ".movie.html")

		// URLエンコードされた元のファイル名を取得
		originalFileName := filepath.Base(originalPath)

		// リンク生成のために親フォルダのパスを取得
		parentURL := filepath.Dir(r.URL.Path)
		if parentURL == "." {
			parentURL = ""
		}
		parentURL = "/" + url.PathEscape(parentURL) + "/"

		// テンプレートに渡すデータを作成
		imageData := VideoTemplateData{
			WS_Title:   originalFileName,
			WS_Link:    "/" + originalPath,
			WS_BaseURL: template.URL(parentURL),
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := movieTmpl.Execute(w, imageData); err != nil {
			log.Printf("テンプレートの実行に失敗しました: %v", err)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusInternalServerError)
			err404Tmpl.Execute(w, NotFoundData{WS_Link: r.URL.Path})
		}
	}
}

// HandleMovieStreamingは動画ファイルをMP4に変換してストリーミングします（FFmpegを使用）。
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
				err404Tmpl.Execute(w, NotFoundData{WS_Link: r.URL.Path})
				return
			}

			// FFmpegコマンドを実行
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
			err404Tmpl.Execute(w, NotFoundData{WS_Link: r.URL.Path})
		}
	}
}
