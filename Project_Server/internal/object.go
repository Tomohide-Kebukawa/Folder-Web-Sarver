// Functions/folder.go:フォルダリストハンドラ:Functions/folder.go

package internal

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"regexp"
)

// ResolvedFoldersを初期化するためのヘルパー関数
func ResolveFolders(folders []string) map[string]string {
	resolved := make(map[string]string)
	for _, folder := range folders {
		absPath, err := filepath.Abs(folder)
		if err != nil {
			log.Printf("フォルダパスの解決に失敗しました %s: %v", folder, err)
			continue
		}
		// ルートフォルダ名をキーとして使用
		resolved[filepath.Base(absPath)] = absPath
	}
	return resolved
}

// isIgnoredは指定されたファイル名が無視リストに含まれているかどうかをチェックします。
func isIgnored(name string, ignores []string) (bool, string) {
	for _, pattern := range ignores {
//		if matched, err := filepath.Match(pattern, name); err == nil && matched {
		if matched, err := regexp.MatchString(pattern, name); err == nil && matched {
			return true, fmt.Sprintf("パターン '%s' に一致しました", pattern)
		}
	}
	if strings.HasPrefix(name, ".") {
		return true, "隠しファイルです"
	}
	return false, ""
}

// HandleObjectRequestはフォルダの内容を一覧表示します。
func HandleObjectRequest(resolvedFolders map[string]string, config *ServerConfig, indexTmpl *template.Template, folderTmpl *template.Template, err404Tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestedPath := getRequestedPath(r)

		// ルートパスの場合
		if requestedPath == "" {
			log.Printf("folder.go: ルートパスがリクエストされました")
			var entries []WS_FileEntry
			for name := range resolvedFolders {
				entries = append(entries, WS_FileEntry{
//					WS_Name:        name,
//					WS_Link:        name,
					WS_Name:        name,
					WS_Link:		strings.ReplaceAll(url.PathEscape(name), "+", "%20") + "/",
//					WS_IsDirectory: true,
				})
			}
			// フォルダとファイルをそれぞれソート
			sort.Slice(entries, func(i, j int) bool {
				return entries[i].WS_Name < entries[j].WS_Name
			})
			sort.Slice(entries, func(i, j int) bool {
				return entries[i].WS_Name < entries[j].WS_Name
			})
			data := FolderData{
				WS_Title:		"Web Server",
				WS_Link:		"/",
				WS_ParentPath:	"",
				WS_Objects:		entries,
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if err := indexTmpl.Execute(w, data); err != nil {
				log.Printf("テンプレートの実行に失敗しました: %v", err)
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				err404Tmpl.Execute(w, NotFoundData{WS_Link: r.URL.Path})
			}
			return
		}

		// ルート以外のパス
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
			if err != nil || !info.IsDir() {
				log.Printf("ファイルの送信: '%s'", fullPath)
				http.ServeFile(w, r, fullPath)
				return
			}
			
			// フォルダの内容を読み込み
			entries, err := os.ReadDir(fullPath)
			if err != nil {
				log.Printf("フォルダの読み込みに失敗しました: '%s' %v", fullPath, err)
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				err404Tmpl.Execute(w, NotFoundData{WS_Link: r.URL.Path})
				return
			}

			// フォルダとファイルのリストを組み立てる
			var fileList	[]WS_FileEntry
			var dirList		[]WS_FileEntry
			for _, entry := range entries {
				ignored, reason := isIgnored(entry.Name(), config.Ignores)
				if ignored {
					log.Printf("Ignores Path: %s (Reason: %s)", filepath.Join(fullPath, entry.Name()), reason)
					continue
				}
				
				info, _ := entry.Info()
				isDir := entry.IsDir()
				
				// フォルダとファイルに分けて処理
				if isDir {
					dirList = append(dirList, WS_FileEntry{
						WS_Name:        entry.Name(),
//						WS_Link:		strings.ReplaceAll(url.PathEscape(entry.Name()), "+", "%20") + "/",
						WS_Link:		url.PathEscape(entry.Name()) + "/",
						WS_LastMod:     info.ModTime().Format("2006-01-02 15:04:05"),
						WS_IsDirectory: true,
//						WS_IconPath:	template.URL(strings.ReplaceAll(url.PathEscape(entry.Name()), "+", "%20") + ".icon"),
						WS_IconPath:	template.URL(url.PathEscape(entry.Name()) + ".icon"),
					})
				} else {
					// 映画と画像を検出
					isMovie := IsMovieFile(entry.Name())
					isImage := isImageFile(filepath.Join(fullPath, entry.Name()))
					
					fileList = append(fileList, WS_FileEntry{
						WS_Name:        entry.Name(),
						WS_Link:        getEntryPath(r.URL.Path, entry.Name(), isMovie, isImage),
//						WS_Size:        formatSize(info.Size()),
						WS_LastMod:     info.ModTime().Format("2006-01-02 15:04:05"),
						WS_IsDirectory: false,
						WS_IsMovie:     isMovie,
						WS_IsImage:     isImage,
						WS_IconPath:	template.URL(url.PathEscape(entry.Name()) + ".icon"),
					})
				}
			}
			
			// フォルダとファイルをそれぞれソート
			sort.Slice(dirList, func(i, j int) bool {
				return dirList[i].WS_Name < dirList[j].WS_Name
			})
			sort.Slice(fileList, func(i, j int) bool {
				return fileList[i].WS_Name < fileList[j].WS_Name
			})
			
			// フォルダのリストとファイルのリストを結合
			var combinedList []WS_FileEntry
			combinedList = append(combinedList, dirList...)
			combinedList = append(combinedList, fileList...)

			// 親フォルダのパスを生成
			parentPath := ""
			if r.URL.Path != "/" {
				parentPath = filepath.Dir(r.URL.Path)
				if parentPath == "." {
					parentPath = "/"
				} else {
					parentPath += "/"
				}
			}

			// テンプレートで利用する変数をまとめる
			data := FolderData{
				WS_Title:		filepath.Base(fullPath),
				WS_Link:		r.URL.Path,
				WS_ParentPath:	parentPath,
				WS_Objects:		combinedList,
			}

			//テンプレートでリスト表示
			log.Printf("フォルダーのリストを表示 '%s'", fullPath)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if err := folderTmpl.Execute(w, data); err != nil {
				log.Printf("テンプレートの実行に失敗しました: %v", err)
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				err404Tmpl.Execute(w, NotFoundData{WS_Link: r.URL.Path})
			}
		} else {
			// 許可されたルートフォルダ以外のパス
			log.Printf("許可されたルートフォルダ以外のパス: '%s'", requestedPath)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			err404Tmpl.Execute(w, NotFoundData{WS_Link: r.URL.Path})
		}
	}
}

// formatSizeはバイト単位のサイズを読みやすい形式に変換します。
func formatSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.2f KB", float64(size)/1024)
	}
	if size < 1024*1024*1024 {
		return fmt.Sprintf("%.2f MB", float64(size)/1024/1024)
	}
	return fmt.Sprintf("%.2f GB", float64(size)/1024/1024/1024)
}

// getEntryPathは、ファイルの種類に応じて適切なパスを返します。
func getEntryPath(basePath, entryName string, isMovie, isImage bool) string {

	// 拡張子を追加する前にURLエンコード
//	encodedEntryName := strings.ReplaceAll(url.PathEscape(entryName), "+", "%20")
	encodedEntryName := url.PathEscape(entryName)

	if isMovie {
		return encodedEntryName + ".movie.html"
	}
	if isImage {
		return encodedEntryName + ".image.html"
	}
	return encodedEntryName
}


//// urlPathEscapeは、url.PathEscapeに頼らずに、ファイルパスのURLエスケープをします。
//func urlPathEscape(path string) string {
//	escapePath := strings.ReplaceAll(path, " ", "%20")
//	escapePath = strings.ReplaceAll(path, "=", "%20")
//}
