package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"Project_Server/internal"
)

func main() {
	// settings.jsonを読み込みます。
	filePath := "./settings.json"
	configFile, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("設定ファイルの読み込みに失敗しました: %v", err)
	}

	var config internal.ServerConfig
	if err := json.Unmarshal(configFile, &config); err != nil {
		log.Fatalf("JSONのパースに失敗しました: %v", err)
	}

	// テンプレートファイルをパースします。
	indexTmpl, err := template.ParseFiles(config.Config.Templates["index"])
	if err != nil {
		log.Fatalf("indexテンプレートファイルのパースに失敗しました: %v", err)
	}
	folderTmpl, err := template.ParseFiles(config.Config.Templates["folder"])
	if err != nil {
		log.Fatalf("folderテンプレートファイルのパースに失敗しました: %v", err)
	}
	imageTmpl, err := template.ParseFiles(config.Config.Templates["image"])
	if err != nil {
		log.Fatalf("imageテンプレートファイルのパースに失敗しました: %v", err)
	}
	imageR2LTmpl, err := template.ParseFiles(config.Config.Templates["imageR2L"])
	if err != nil {
		log.Fatalf("imageR2Lテンプレートファイルのパースに失敗しました: %v", err)
	}
	// movieテンプレートを追加
	movieTmpl, err := template.ParseFiles(config.Config.Templates["movie"])
	if err != nil {
		log.Fatalf("movieテンプレートファイルのパースに失敗しました: %v", err)
	}
	// 404テンプレートを追加
	err404Tmpl, err := template.ParseFiles("./templates/404.html")
	if err != nil {
		log.Fatalf("404テンプレートファイルのパースに失敗しました: %v", err)
	}

	// サーバー起動時にフォルダパスを解決し、マップにキャッシュ
	resolvedFolders := internal.ResolveFolders(config.Folders)

	// HTTPハンドラを設定します。
	http.HandleFunc("/icon/", internal.HandleIconRequest(resolvedFolders, &config, err404Tmpl))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		requestedPath := getRequestedPath(r)

		// .iconで終わるリクエストはicon.goのハンドラにリダイレクト
		if strings.HasSuffix(requestedPath, ".icon") {
			internal.HandleIconRequest(resolvedFolders, &config, err404Tmpl)(w, r)
			return
		}

		// .image.htmlで終わるリクエストはimage.goのハンドラにリダイレクト
		if strings.HasSuffix(requestedPath, ".image.html") {
			internal.HandleImageRequest(resolvedFolders, &config, imageTmpl, imageR2LTmpl, err404Tmpl)(w, r)
			return
		}

		// .movie.htmlで終わるリクエストはmovie.goのハンドラにリダイレクト
		if strings.HasSuffix(requestedPath, ".movie.html") {
			internal.HandleMoviePage(resolvedFolders, &config, movieTmpl, err404Tmpl)(w, r)
			return
		}
		
		// .htmlで終わらない動画ファイルへのリクエストはmovie.goのハンドラにリダイレクト
		if !strings.HasSuffix(requestedPath, ".html") && internal.IsMovieFile(requestedPath) {
			internal.HandleMovieStreaming(resolvedFolders, &config, err404Tmpl)(w, r)
			return
		}

		// hls/で始まるリクエストはmovie.goのハンドラにリダイレクト
		if strings.HasPrefix(requestedPath, "hls/") {
			internal.HandleMovieStreaming(resolvedFolders, &config, err404Tmpl)(w, r)
			return
		}

		// それ以外のすべてのリクエストはobject.goのハンドラに渡す
		internal.HandleObjectRequest(resolvedFolders, &config, indexTmpl, folderTmpl, err404Tmpl)(w, r)
	})

	// Webサーバーを起動します。
	port := fmt.Sprintf(":%d", config.Config.Server.Port)
	fmt.Printf("Web Server Start (port:%s)...\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
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
