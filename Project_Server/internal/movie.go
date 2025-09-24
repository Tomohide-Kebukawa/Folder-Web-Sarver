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


	"fmt"
	"io"
)

func HandleMovieFFmpeg(w http.ResponseWriter, r *http.Request, filePath string, config *ServerConfig) {

	// ffmpeg コマンド
	cmd := exec.Command("ffmpeg",
		"-i", filePath,
		"-c:v", "libx264",
		"-f", "mp4",
		"-movflags", "frag_keyframe+empty_moov+default_base_moof",
		"pipe:1")

	// 標準出力のパイプを取得
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Println("Movie: StdoutPipe error:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// HTTPレスポンスヘッダーの設定
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Transfer-Encoding", "chunked")

	// FFmpegプロセスを開始
	if err := cmd.Start(); err != nil {
		log.Println("Movie: ffmpeg Start error:", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// パイプから読み込んだデータを直接HTTPレスポンスに書き込み
	_, err = io.Copy(w, stdout)
	if err != nil {
		log.Println("Movie: io.Copy error:", err)
		// エラーハンドリングは必要に応じて
	}

	// プロセスの終了を待機し、リソースを解放
	if err := cmd.Wait(); err != nil {
		log.Println("Movie: ffmpeg Wait error:", err)
	}

	fmt.Println("Movie: Streaming complete.")
}




// HandleMoviePageは動画再生ページをレンダリングします。
func HandleMoviePage(resolvedFolders map[string]string, config *ServerConfig, movieTmpl *template.Template, err404Tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestedPath := getRequestedPath(r)

		log.Printf("Movie: 動画再生ページ: '%s'", requestedPath)

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
			log.Printf("Movie: テンプレートの実行に失敗しました: %v", err)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusInternalServerError)
			err404Tmpl.Execute(w, NotFoundData{WS_Link: r.URL.Path})
		}
	}
}

// HandleMovieStreamingは動画ファイルをMP4に変換してストリーミングする
func HandleMovieStreaming(resolvedFolders map[string]string, config *ServerConfig, err404Tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		
		// リクエストパスの取り出し
		requestedPath := getRequestedPath(r)
		
		log.Printf("Movie: リクエスト受取: '%s'", requestedPath)
		
		// リクエストされたファイルパスを得る
		var fullPath string
//		pathParts := strings.Split(requestedPath, string(os.PathSeparator))
		pathParts := strings.Split(requestedPath, "/") // フォルダー階層お名称を配列化
		firstFolder := pathParts[0] //最初の階層を取り出す
		if resolvedPath, ok := resolvedFolders[firstFolder]; ok { //最初の階層がFoldersで設定されたフォルダー群にあるか調べる
			fullPath = resolvedPath // リクエストされたフォルダーのパス
			// 残りのフォルダー階層を繋げてリクエストされたファイルの絶対パスを得る
			if len(pathParts) > 1 {
				subPath := filepath.Join(pathParts[1:]...)
				fullPath = filepath.Join(resolvedPath, subPath)
			}
		}
		
		// リクエストされたファイルの情報
		fileInfo, fileErr := os.Stat(fullPath)

		log.Printf("Movie: リクエストファイルパス: '%s'", requestedPath)
		
		// ファイルが存在しないときは404を返す
		if fileErr != nil || !fileInfo.Mode().IsRegular() || !IsMovieFile(fullPath) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			err404Tmpl.Execute(w, NotFoundData{WS_Link: r.URL.Path})
			return
		}

		// MP4はそのまま送信			
		if strings.HasSuffix(strings.ToLower(fullPath), ".mp4") {
			log.Printf("Movie: MP4ファイルの送信: '%s'", fullPath)
			http.ServeFile(w, r, fullPath)
			return
		}

		HandleMovieFFmpeg(w, r, fullPath, config)
			
	}
}
