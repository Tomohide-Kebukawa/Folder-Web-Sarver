package Functions

import (
	"fmt"
	"html/template"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ImageTemplateDataは画像表示テンプレートに渡すデータを定義します。
type ImageTemplateData struct {
	WS_Title     string
	WS_Path      string
	CurrentIndex int
	ImagePaths   []string
	WS_BaseURL   string
}

// NotFoundDataは404テンプレートに渡すデータを定義します。
type NotFoundData struct {
	WS_Path string
}

// HandleImageRequestは画像ビューアのHTMLを返します。
func HandleImageRequest(resolvedFolders map[string]string, config *ServerConfig, imageTmpl *template.Template, imageR2LTmpl *template.Template, err404Tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestedPath := getRequestedPath(r)

		// 仮想パスから元のファイルパスを特定
		originalPath := strings.TrimSuffix(requestedPath, ".image.html")

		// 許可されたルートフォルダを基に、完全なファイルパスを再構築
		var fullPath string
		pathParts := strings.Split(originalPath, string(os.PathSeparator))
		firstFolder := pathParts[0]

		if resolvedPath, ok := resolvedFolders[firstFolder]; ok {
			fullPath = resolvedPath
			if len(pathParts) > 1 {
				subPath := filepath.Join(pathParts[1:]...)
				fullPath = filepath.Join(resolvedPath, subPath)
			}

			info, err := os.Stat(fullPath)
			if err == nil && info.Mode().IsRegular() && isImageFile(fullPath) {
				// 元の画像ファイルが存在する場合、テンプレートを返す
				parentDir := filepath.Dir(fullPath)
				dirEntries, err := os.ReadDir(parentDir)
				if err != nil {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.WriteHeader(http.StatusInternalServerError)
					err404Tmpl.Execute(w, NotFoundData{WS_Path: r.URL.Path})
					return
				}

				var imageFileEntries []os.DirEntry
				for _, entry := range dirEntries {
					if !entry.IsDir() {
						if ignored, reason := isIgnored(entry.Name(), config.Ignores); ignored {
							log.Printf("Ignores Path: %s (Reason: %s)", filepath.Join(parentDir, entry.Name()), reason)
							continue
						}
					}
					if isImageFile(filepath.Join(parentDir, entry.Name())) {
						imageFileEntries = append(imageFileEntries, entry)
					}
				}

				_, errR2L := os.Stat(filepath.Join(parentDir, "__option_R2L__"))

				sort.Slice(imageFileEntries, func(i, j int) bool {
					return imageFileEntries[i].Name() < imageFileEntries[j].Name()
				})

				var imagePaths []string
				currentIndex := -1
				for idx, entry := range imageFileEntries {
					imagePaths = append(imagePaths, url.PathEscape(entry.Name()))
					if entry.Name() == filepath.Base(fullPath) {
						currentIndex = idx
					}
				}

				// WS_BaseURLはURLエンコードされたフォルダパスを返す
				parentURL := filepath.Dir(r.URL.Path) + "/"

				imageData := ImageTemplateData{
					WS_Title:     filepath.Base(fullPath),
					WS_Path:      r.URL.Path,
					CurrentIndex: currentIndex,
					ImagePaths:   imagePaths,
					WS_BaseURL:   parentURL,
				}

				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				if errR2L == nil {
					if err := imageR2LTmpl.Execute(w, imageData); err != nil {
						log.Printf("テンプレートの実行に失敗しました: %v", err)
						w.Header().Set("Content-Type", "text/html; charset=utf-8")
						w.WriteHeader(http.StatusInternalServerError)
						err404Tmpl.Execute(w, NotFoundData{WS_Path: r.URL.Path})
					}
				} else {
					if err := imageTmpl.Execute(w, imageData); err != nil {
						log.Printf("テンプレートの実行に失敗しました: %v", err)
						w.Header().Set("Content-Type", "text/html; charset=utf-8")
						w.WriteHeader(http.StatusInternalServerError)
						err404Tmpl.Execute(w, NotFoundData{WS_Path: r.URL.Path})
					}
				}
				return
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		err404Tmpl.Execute(w, NotFoundData{WS_Path: r.URL.Path})
	}
}

// isImageFileはファイルが画像ファイルであるかチェックします。
func isImageFile(path string) bool {
	mimeType := mime.TypeByExtension(filepath.Ext(path))
	return strings.HasPrefix(mimeType, "image/")
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
