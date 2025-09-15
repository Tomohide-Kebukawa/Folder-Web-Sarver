package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"html/template"
	"mime"
	"sort"
	"encoding/base64"
	"regexp"
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

// Objectはファイルまたはフォルダーの情報を保持します。
type Object struct {
	Name string
	Type string // "folder" or "file"
	Link string // Added for custom links
	IconPath template.URL // Custom icon path
}

// TemplateDataはテンプレートに渡すデータを定義します。
type TemplateData struct {
	WS_Title string
	WS_Path  string
	WS_Objects []Object
}

// ImageTemplateDataは画像表示テンプレートに渡すデータを定義します。
type ImageTemplateData struct {
	WS_Title string
	WS_Path string
	CurrentIndex int
	ImagePaths []string
}

// getRequestedPathはリクエストされたパスを正規化し、セキュリティ上の問題を回避します。
func getRequestedPath(r *http.Request) string {
	path := strings.TrimPrefix(r.URL.Path, "/")
	path = filepath.Clean(path)
	if path == "." {
		return ""
	}
	return path
}

// resolveFoldersは、settings.jsonに記載されたパスをキーとし、存在チェックを行ったマップを返します。
func resolveFolders(folders []string) map[string]string {
	resolved := make(map[string]string)
	for _, folderPath := range folders {
		cleanPath := filepath.Clean(folderPath)
		// フォルダが存在するかどうかを確認
		if _, err := os.Stat(cleanPath); err == nil {
			baseName := filepath.Base(cleanPath)
			resolved[baseName] = cleanPath
		} else {
			log.Printf("指定されたフォルダが見つからないか、アクセスできません: %s", cleanPath)
		}
	}
	return resolved
}

// isImageFileはファイルが画像ファイルであるかチェックします。
func isImageFile(path string) bool {
	mimeType := mime.TypeByExtension(filepath.Ext(path))
	return strings.HasPrefix(mimeType, "image/")
}

// getIconBase64は、getIconツールを実行してBase64エンコードされたPNGを返します。
func getIconBase64(filePath string) (string, error) {
	cmd := exec.Command("./getIcon", filePath)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get icon for %s: %v", filePath, err)
	}
	return strings.TrimSpace(string(output)), nil
}

// isIgnoredは、ファイル名がignoresリストに含まれているか正規表現を使ってチェックします。
func isIgnored(name string, ignores []string) (bool, string) {
	for _, ignoredPattern := range ignores {
		// ワイルドカード '*' を正規表現の '.' に変換
		pattern := strings.ReplaceAll(ignoredPattern, "*", ".*")
		// パターンをコンパイル
		matched, err := regexp.MatchString("^" + pattern + "$", name)
		if err == nil && matched {
			return true, ignoredPattern // 除外された理由を返す
		}
	}
	// "__option_R2L__" ファイルは常に除外
	if name == "__option_R2L__" {
		return true, "__option_R2L__"
	}
	return false, ""
}

func main() {
	// settings.jsonを読み込みます。
	filePath := "./settings.json"
	configFile, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("設定ファイルの読み込みに失敗しました: %v", err)
	}

	var config ServerConfig
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


	// サーバー起動時にフォルダパスを解決し、マップにキャッシュ
	resolvedFolders := resolveFolders(config.Folders)
	
	// アイコン取得のためのハンドラを設定します。
	http.HandleFunc("/icon/", func(w http.ResponseWriter, r *http.Request) {
		requestedPath := getRequestedPath(r)
		
		// 許可されたルートフォルダを基に、完全なファイルパスを再構築
		var fullPath string
		pathParts := strings.Split(requestedPath, string(os.PathSeparator))
		firstFolder := pathParts[0]
		
		if resolvedPath, ok := resolvedFolders[firstFolder]; ok {
			fullPath = resolvedPath
			if len(pathParts) > 1 {
				subPath := filepath.Join(pathParts[1:]...)
				fullPath = filepath.Join(resolvedPath, subPath)
			}
		} else {
			// ルートフォルダに存在しない場合は404
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		
		// getIconツールを実行してBase64データを取得
		base64Data, err := getIconBase64(fullPath)
		if err != nil {
			log.Printf("アイコンの取得に失敗しました: %v", err)
			http.Error(w, "内部サーバーエラー", http.StatusInternalServerError)
			return
		}
		
		// Content-Typeをimage/pngに設定し、Base64データをデコードして書き込む
		w.Header().Set("Content-Type", "image/png")
		data, err := base64.StdEncoding.DecodeString(base64Data)
		if err != nil {
			log.Printf("Base64のデコードに失敗しました: %v", err)
			http.Error(w, "内部サーバーエラー", http.StatusInternalServerError)
			return
		}
		w.Write(data)
	})


	// HTTPハンドラを設定します。
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		requestedPath := getRequestedPath(r)
		
		// .iconで終わるリクエストを処理
		if strings.HasSuffix(requestedPath, ".icon") {
			// .iconを取り除いて元のファイルパスを取得
			originalPath := strings.TrimSuffix(requestedPath, ".icon")
			
			// パスを解決して絶対パスを構築
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
						http.Error(w, "Internal Server Error", http.StatusInternalServerError)
						return
					}
					
					// image/pngとしてデータを返す
					w.Header().Set("Content-Type", "image/png")
					data, err := base64.StdEncoding.DecodeString(base64Data)
					if err != nil {
						log.Printf("Base64のデコードに失敗しました: %v", err)
						http.Error(w, "Internal Server Error", http.StatusInternalServerError)
						return
					}
					w.Write(data)
					return
				}
			}
			// ファイル/フォルダが存在しない、または無効なリクエストの場合
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		
		var objects []Object
		var tmplToExecute *template.Template
		var title string

		// ルートURLへのアクセスの場合
		if requestedPath == "" {
			title = "ホーム"
			tmplToExecute = indexTmpl
			// settings.jsonに記載されたトップレベルのフォルダーをリスト
			for folderName := range resolvedFolders {
				// ignoresリストに含まれていれば除外
				if ignored, reason := isIgnored(folderName, config.Ignores); ignored {
					log.Printf("除外されたパス: %s (理由: %s)", resolvedFolders[folderName], reason)
					continue
				}

				obj := Object{Name: folderName + "/", Type: "folder", Link: folderName + "/"}
				obj.IconPath = template.URL("/" + obj.Link + ".icon")
				objects = append(objects, obj)
			}
			// オブジェクトを名前でソート
			sort.Slice(objects, func(i, j int) bool {
				return objects[i].Name < objects[j].Name
			})
		} else {
			// 特定のフォルダーへのアクセスの場合
			// ルートフォルダーからリクエストされたパスへのマッピングを試みる
			var targetBasePath string
			
			// 異なるボリュームへのアクセスを考慮するため、settings.jsonのフォルダーから検索
			pathParts := strings.Split(requestedPath, string(os.PathSeparator))
			firstFolder := pathParts[0]

			if resolvedPath, ok := resolvedFolders[firstFolder]; ok {
				// 有効なパスの場合、残りのパスを結合
				targetBasePath = resolvedPath
				if len(pathParts) > 1 {
					subPath := filepath.Join(pathParts[1:]...)
					targetBasePath = filepath.Join(resolvedPath, subPath)
				}
				
				// フォルダの存在と種類を確認
				info, err := os.Stat(targetBasePath)
				if err != nil {
					// 404エラーの場合、仮想パスの可能性をチェック
					if strings.HasSuffix(requestedPath, ".image.html") {
						// 仮想パスから元のファイルパスを特定
						originalPath := strings.TrimSuffix(targetBasePath, ".image.html")
						info, err := os.Stat(originalPath)
						if err == nil && info.Mode().IsRegular() && isImageFile(originalPath) {
							// 元の画像ファイルが存在する場合、テンプレートを返す
							parentDir := filepath.Dir(originalPath)
							dirEntries, err := os.ReadDir(parentDir)
							if err != nil {
								http.Error(w, "ディレクトリの読み込みができません", http.StatusInternalServerError)
								return
							}
							
							var imageFileEntries []os.DirEntry
							for _, entry := range dirEntries {
								// ignoresリストに含まれていれば除外
								if !entry.IsDir() {
									if ignored, reason := isIgnored(entry.Name(), config.Ignores); ignored {
										log.Printf("除外されたパス: %s (理由: %s)", filepath.Join(parentDir, entry.Name()), reason)
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
								fileServerPath := filepath.Join("/", pathParts[0], strings.TrimPrefix(filepath.Join(strings.Join(pathParts[1:len(pathParts)-1], "/"), entry.Name()), "/"))
								imagePaths = append(imagePaths, fileServerPath)
								if entry.Name() == filepath.Base(originalPath) {
									currentIndex = idx
								}
							}
							
							imageData := ImageTemplateData{
								WS_Title: filepath.Base(originalPath),
								WS_Path: r.URL.Path,
								CurrentIndex: currentIndex,
								ImagePaths: imagePaths,
							}
							
							w.Header().Set("Content-Type", "text/html; charset=utf-8")
							if errR2L == nil {
								if err := imageR2LTmpl.Execute(w, imageData); err != nil {
									log.Printf("テンプレートの実行に失敗しました: %v", err)
									http.Error(w, "内部サーバーエラー", http.StatusInternalServerError)
								}
							} else {
								if err := imageTmpl.Execute(w, imageData); err != nil {
									log.Printf("テンプレートの実行に失敗しました: %v", err)
									http.Error(w, "内部サーバーエラー", http.StatusInternalServerError)
								}
							}
							return
						}
					}
					http.Error(w, fmt.Sprintf("404 Not Found: %s", r.URL.Path), http.StatusNotFound)
					return
				}
				
				// index.html or index.htmへの直接アクセスを処理
				if strings.HasSuffix(strings.ToLower(requestedPath), "index.html") || strings.HasSuffix(strings.ToLower(requestedPath), "index.htm") {
					// ファイルが存在すれば直接提供
					if info.Mode().IsRegular() {
						file, err := os.Open(targetBasePath)
						if err != nil {
							http.Error(w, "ファイルのオープンに失敗しました", http.StatusInternalServerError)
							return
						}
						defer file.Close()
						
						// リダイレクトを防ぐためにhttp.ServeContentを使用
						http.ServeContent(w, r, filepath.Base(targetBasePath), info.ModTime(), file)
						return
					}
				}
				
				// リクエストされたパスがファイルであり、画像テンプレートでない場合
				if info.Mode().IsRegular() && !strings.HasSuffix(requestedPath, ".image.html") {
					http.ServeFile(w, r, targetBasePath)
					return
				}
				
				if info.IsDir() {
					// サブディレクトリ内のフォルダーとファイルをリストアップ
					files, err := os.ReadDir(targetBasePath)
					if err != nil {
						log.Printf("フォルダの読み込みに失敗しました: %v", err)
						http.Error(w, "内部サーバーエラー", http.StatusInternalServerError)
						return
					}

					
					for _, file := range files {
						if ignored, reason := isIgnored(file.Name(), config.Ignores); ignored {
							fullPath := filepath.Join(targetBasePath, file.Name())
							log.Printf("除外されたパス: %s (理由: %s)", fullPath, reason)
							continue
						}
						
						obj := Object{Name: file.Name()}
						
						if file.IsDir() {
							obj.Link = file.Name() + "/"
							obj.Type = "folder"
							obj.IconPath = template.URL(r.URL.Path + obj.Link + ".icon")
						} else {
							obj.Type = "file"
							if isImageFile(filepath.Join(targetBasePath, file.Name())) {
								obj.Link = file.Name() + ".image.html"
							} else {
								obj.Link = file.Name()
							}
							obj.IconPath = template.URL(r.URL.Path + file.Name() + ".icon")
						}
						objects = append(objects, obj)
					}
					// オブジェクトを名前でソート
					sort.Slice(objects, func(i, j int) bool {
						return objects[i].Name < objects[j].Name
					})
					title = filepath.Base(targetBasePath)
					tmplToExecute = folderTmpl
				} else {
					// その他、シンボリックリンクなど未対応のタイプ
					http.Error(w, fmt.Sprintf("サポートされていないファイルの種類: %s", r.URL.Path), http.StatusForbidden)
					return
				}
			} else {
				// ルートフォルダーに存在しないパスの場合、404エラー
				http.Error(w, fmt.Sprintf("404 Not Found: %s", r.URL.Path), http.StatusNotFound)
				return
			}
		}
		
		// テンプレートに渡すデータを作成します。
		data := TemplateData{
			WS_Title: title,
			WS_Path:  r.URL.Path,
			WS_Objects: objects,
		}

		// テンプレートを実行し、HTMLをレスポンスとして返します。
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmplToExecute.Execute(w, data); err != nil {
			log.Printf("テンプレートの実行に失敗しました: %v", err)
			http.Error(w, "内部サーバーエラー", http.StatusInternalServerError)
		}
	})
	
	// Webサーバーを起動します。
	port := fmt.Sprintf(":%d", config.Config.Server.Port)
	fmt.Printf("Webサーバーをポート%sで起動します...\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}
