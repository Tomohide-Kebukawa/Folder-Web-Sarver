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
	"regexp"
	"sort"
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

// Objectはファイルまたはフォルダの情報を保持します。
type Object struct {
	WS_Name     string
	Type        string // "folder" または "file"
	WS_Link     string
	IconPath    template.URL
	WS_IconPath template.URL
}

// TemplateDataはテンプレートに渡されるデータを定義します。
type TemplateData struct {
	WS_Title   string
	WS_Path    string
	WS_BaseURL string
	WS_Objects []Object
}

// NotFoundDataは404テンプレートに渡されるデータを定義します。
type NotFoundData struct {
	WS_Path string
}

// HandleFolderRequestはフォルダ内のファイルリストを返します。
func HandleFolderRequest(resolvedFolders map[string]string, config *ServerConfig, indexTmpl *template.Template, folderTmpl *template.Template, err404Tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestedPath := getRequestedPath(r)

		var objects []Object
		var tmplToExecute *template.Template
		var title string

		// ルートURLへのアクセス
		if requestedPath == "" {
			title = "ホーム"
			tmplToExecute = indexTmpl
			// settings.jsonからトップレベルのフォルダをリストアップ
			for folderName := range resolvedFolders {
				// ignoresリストに含まれている場合は除外
				if ignored, reason := isIgnored(folderName, config.Ignores); ignored {
					log.Printf("除外されたパス: %s (理由: %s)", filepath.Join(resolvedFolders[folderName]), reason)
					continue
				}

				encodedLink := url.PathEscape(folderName) + "/"
				iconPath := template.URL(fmt.Sprintf("/icon/%s/", url.PathEscape(folderName)))

				obj := Object{WS_Name: folderName, Type: "folder", WS_Link: encodedLink, IconPath: iconPath}
				objects = append(objects, obj)
			}
		} else {
			// 特定のフォルダへのアクセス
			var targetBasePath string

			// settings.jsonのフォルダから検索して、異なるボリュームを処理
			pathParts := strings.Split(requestedPath, string(os.PathSeparator))
			firstFolder := pathParts[0]

			if resolvedPath, ok := resolvedFolders[firstFolder]; ok {
				targetBasePath = resolvedPath
				if len(pathParts) > 1 {
					subPath := filepath.Join(pathParts[1:]...)
					targetBasePath = filepath.Join(resolvedPath, subPath)
				}

				// フォルダの存在とタイプを確認
				info, err := os.Stat(targetBasePath)
				if err != nil || !info.IsDir() {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.WriteHeader(http.StatusNotFound)
					err404Tmpl.Execute(w, NotFoundData{WS_Path: r.URL.Path})
					return
				}

				// サブディレクトリ内のフォルダとファイルをリストアップ
				files, err := os.ReadDir(targetBasePath)
				if err != nil {
					log.Printf("フォルダの読み取りに失敗しました: %v", err)
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.WriteHeader(http.StatusInternalServerError)
					err404Tmpl.Execute(w, NotFoundData{WS_Path: r.URL.Path})
					return
				}

				for _, file := range files {
					if ignored, reason := isIgnored(file.Name(), config.Ignores); ignored {
						fullPath := filepath.Join(targetBasePath, file.Name())
						log.Printf("除外されたパス: %s (理由: %s)", fullPath, reason)
						continue
					}

					obj := Object{WS_Name: file.Name()}

					// ファイル名をURLエンコード
					encodedName := url.PathEscape(file.Name())
					obj.WS_Link = encodedName

					// ファイルタイプを判別し、正しいリンクを作成
					if file.IsDir() {
						log.Printf("Type=folder '%s' ", file.Name())
						obj.WS_Link = obj.WS_Link + "/"
						obj.Type = "folder"
					} else {
						log.Printf("Type=file '%s' ", file.Name())
						obj.Type = "file"
						if isImageFile(filepath.Join(targetBasePath, file.Name())) {
							obj.WS_Link = obj.WS_Link + ".image.html"
						} else if IsMovieFile(filepath.Join(targetBasePath, file.Name())) {
							obj.WS_Link = obj.WS_Link + ".movie.html"
						}
					}
					
					// IconPathとWS_IconPathを設定
					obj.WS_IconPath = template.URL(obj.WS_Link + ".icon")
					obj.IconPath = template.URL(fmt.Sprintf("/icon/%s/%s", url.PathEscape(requestedPath), obj.WS_Link))
					
					objects = append(objects, obj)
				}
				title = filepath.Base(targetBasePath)
				tmplToExecute = folderTmpl
			} else {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusNotFound)
				err404Tmpl.Execute(w, NotFoundData{WS_Path: r.URL.Path})
				return
			}
		}

		// オブジェクトを名前でソート
		sort.Slice(objects, func(i, j int) bool {
			return objects[i].WS_Name < objects[j].WS_Name
		})

		data := TemplateData{
			WS_Title:   title,
			WS_Path:    r.URL.Path,
			WS_BaseURL: "/" + url.PathEscape(requestedPath) + "/",
			WS_Objects: objects,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmplToExecute.Execute(w, data); err != nil {
			log.Printf("テンプレートの実行に失敗しました: %v", err)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusInternalServerError)
			err404Tmpl.Execute(w, NotFoundData{WS_Path: r.URL.Path})
		}
	}
}

// ResolveFoldersはsettings.jsonで指定されたパスを解決し、存在を確認してマップを返します。
func ResolveFolders(folders []string) map[string]string {
	resolved := make(map[string]string)
	for _, folderPath := range folders {
		cleanPath := filepath.Clean(folderPath)
		if _, err := os.Stat(cleanPath); err == nil {
			baseName := filepath.Base(cleanPath)
			resolved[baseName] = cleanPath
		} else {
			log.Printf("指定されたフォルダが見つからないかアクセスできません: %s", cleanPath)
		}
	}
	return resolved
}

// isIgnoredは正規表現を使用して、ファイル名がignoresリストにあるかどうかをチェックします。
func isIgnored(name string, ignores []string) (bool, string) {
	for _, ignoredPattern := range ignores {
		pattern := strings.ReplaceAll(ignoredPattern, "*", ".*")
		matched, err := regexp.MatchString("^" + pattern + "$", name)
		if err == nil && matched {
			return true, ignoredPattern
		}
	}
	if name == "__option_R2L__" {
		return true, "__option_R2L__"
	}
	return false, ""
}

// isImageFileはファイルが画像ファイルであるかどうかをチェックします。
func isImageFile(path string) bool {
	mimeType := mime.TypeByExtension(filepath.Ext(path))
	return strings.HasPrefix(mimeType, "image/")
}

// IsMovieFileはファイルが動画ファイルであるかどうかをチェックします。
func IsMovieFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	log.Printf("ファイル: '%s' (抽出された拡張子: '%s')", path, ext)
	// 一般的に使用される動画拡張子をチェック
	return ext == ".mkv" || ext == ".mov" || ext == ".avi" || ext == ".webm" || ext == ".mp4" || ext == ".wmv" || ext == ".flv"
}

// getRequestedPathはセキュリティ上の問題を防止するために、リクエストされたパスを正規化します。
func getRequestedPath(r *http.Request) string {
	path := strings.TrimPrefix(r.URL.Path, "/")
	path, _ = url.PathUnescape(path) // URLデコード
	path = filepath.Clean(path)
	if path == "." {
		return ""
	}
	return path
}
