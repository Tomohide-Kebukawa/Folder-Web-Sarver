package Functions

import (
	"encoding/base64"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ServerConfigはsettings.jsonの構造を定義します。
type ServerConfig struct {
	Config struct {
		Server struct {
			Port int `json:"port"`
		} `json:"server"`
		Templates map[string]string `json:"templates"`
	} `json:"config"`
	Folders []string `json:"folders"`
	Ignores []string `json:"ignores"`
}

// NotFoundDataは404テンプレートに渡すデータを定義します。
type NotFoundData struct {
	WS_Path string
}

// HandleIconRequestは`.icon`リクエストを処理してアイコン画像を返します。
func HandleIconRequest(resolvedFolders map[string]string, config *ServerConfig, err404Tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestedPath := getRequestedPath(r)

		// .iconで終わるリクエストを処理
		if strings.HasSuffix(requestedPath, ".icon") {
			// .iconを取り除いて元のファイルパスを取得
			originalPath := strings.TrimSuffix(requestedPath, ".icon")
			handleIconFile(w, r, originalPath, resolvedFolders, config, err404Tmpl)
			return
		}

		// `/icon/`で始まるリクエストを処理
		if strings.HasPrefix(r.URL.Path, "/icon/") {
			requestedPath = strings.TrimPrefix(r.URL.Path, "/icon/")
			handleIconFile(w, r, requestedPath, resolvedFolders, config, err404Tmpl)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		err404Tmpl.Execute(w, NotFoundData{WS_Path: r.URL.Path})
	}
}

// handleIconFileは、指定されたパスのアイコンを返します。
func handleIconFile(w http.ResponseWriter, r *http.Request, originalPath string, resolvedFolders map[string]string, config *ServerConfig, err404Tmpl *template.Template) {
	var fullPath string
	pathParts := strings.Split(originalPath, string(os.PathSeparator))
	firstFolder := pathParts[0]

	if resolvedPath, ok := resolvedFolders[firstFolder]; ok {
		fullPath = resolvedPath
		if len(pathParts) > 1 {
			subPath := filepath.Join(pathParts[1:]...)
			fullPath = filepath.Join(resolvedPath, subPath)
		}

		// パスが存在し、それがファイルまたはフォルダーであることを確認
		info, err := os.Stat(fullPath)
		if err == nil && (info.Mode().IsRegular() || info.IsDir()) {
			// getIconツールを実行してBase64データを取得
			base64Data, err := getIconBase64(fullPath)
			if err != nil {
				log.Printf("アイコンの取得に失敗しました: %v", err)
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				err404Tmpl.Execute(w, NotFoundData{WS_Path: r.URL.Path})
				return
			}

			// image/pngとしてデータを返す
			w.Header().Set("Content-Type", "image/png")
			data, err := base64.StdEncoding.DecodeString(base64Data)
			if err != nil {
				log.Printf("Base64のデコードに失敗しました: %v", err)
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				err404Tmpl.Execute(w, NotFoundData{WS_Path: r.URL.Path})
				return
			}
			w.Write(data)
			return
		}
	}

	// ファイル/フォルダが存在しない、または無効なリクエストの場合
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	err404Tmpl.Execute(w, NotFoundData{WS_Path: r.URL.Path})
}

// getIconBase64は、getIconツールを実行してBase64エンコードされたPNGを返します。
func getIconBase64(filePath string) (string, error) {
	cmd := exec.Command("./Libraries/getIcon", filePath)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get icon for %s: %v", filePath, err)
	}
	return strings.TrimSpace(string(output)), nil
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