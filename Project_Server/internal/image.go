// Functions/image.go:画像ビューアハンドラ:Functions/image.go

package internal

import (
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"fmt"
	"os/exec"
	"regexp"

	"strconv"
	"github.com/google/uuid"

)


// HandleImageRequestは画像ビューアのHTMLを返します。
func HandleImageRequest(resolvedFolders map[string]string, config *ServerConfig, imageTmpl *template.Template, imageR2LTmpl *template.Template, image360vrTmpl *template.Template, err404Tmpl *template.Template) http.HandlerFunc {
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
					err404Tmpl.Execute(w, NotFoundData{WS_Link: r.URL.Path})
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

				_, errR2L	:= os.Stat(filepath.Join(parentDir, "__option_R2L__"))
				_, err360vr	:= os.Stat(filepath.Join(parentDir, "__option_360VR__"))

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

				imageData := ImageData{
					WS_Title:			template.URL(filepath.Base(fullPath)),
					WS_Link:			template.URL(r.URL.Path),
					WS_CurrentIndex:	currentIndex,
					WS_ImagePaths:		imagePaths,
					WS_ImageFile:		filepath.Base(fullPath),
					WS_BaseURL:			template.URL(parentURL),
				}

				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				if errR2L == nil {
					if err := imageR2LTmpl.Execute(w, imageData); err != nil {
						log.Printf("テンプレートの実行に失敗しました: %v", err)
						w.Header().Set("Content-Type", "text/html; charset=utf-8")
						w.WriteHeader(http.StatusInternalServerError)
						err404Tmpl.Execute(w, NotFoundData{WS_Link: r.URL.Path})
					}
				} else if err360vr == nil {
					if err := image360vrTmpl.Execute(w, imageData); err != nil {
						log.Printf("テンプレートの実行に失敗しました: %v", err)
						w.Header().Set("Content-Type", "text/html; charset=utf-8")
						w.WriteHeader(http.StatusInternalServerError)
						err404Tmpl.Execute(w, NotFoundData{WS_Link: r.URL.Path})
					}
				} else {
					if err := imageTmpl.Execute(w, imageData); err != nil {
						log.Printf("テンプレートの実行に失敗しました: %v", err)
						w.Header().Set("Content-Type", "text/html; charset=utf-8")
						w.WriteHeader(http.StatusInternalServerError)
						err404Tmpl.Execute(w, NotFoundData{WS_Link: r.URL.Path})
					}
				}
				return
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		err404Tmpl.Execute(w, NotFoundData{WS_Link: r.URL.Path})
	}
}




// resizeAndSaveWithUUID は、指定された画像をリサイズし、
// UUIDをファイル名として指定のフォルダに保存します。
// 成功した場合は新しいファイルのフルパスを返します。
func imageResize(inputPath string, size int, config *ServerConfig) (string, error) {

	// 作業用フォルダー
	outputDir := config.Config.Temporary

	// UUIDを生成し、新しいファイル名を決定
	newUUID := uuid.New().String()
	
	// 入力ファイルの拡張子を取得
	ext := filepath.Ext(inputPath)
	if ext == "" {
		// 拡張子がない場合は、jpegとして扱うなどの代替処理が必要になる場合がある
		// ここでは、デフォルトで.pngを使用する
		ext = ".png" 
	}
	
	newFileName := newUUID + ext
	
	// 保存先ファイルのフルパスを作成
	// filepath.JoinでOSに依存しない適切なパスを構築
	outputPath := filepath.Join(outputDir, newFileName)
	
	// sipsコマンドを組み立てる
	// -Z (または --maxSize) は、縦横の長い辺が指定サイズになるように画像をリサイズします
	// --out は出力先を指定します
	sizeStr := strconv.Itoa(size)
	
	cmd := exec.Command("sips", "-Z", sizeStr, inputPath, "--out", outputPath)

	// sipsコマンド実行 (標準出力/エラーは無視)
	err := cmd.Run()
	if err != nil {
		// sipsコマンドが失敗した場合（例: 入力ファイルが存在しない、出力ディレクトリが存在しないなど）
		return "", fmt.Errorf("sipsコマンド実行失敗: %w", err)
	}

	// 4. 成功した場合、新しいファイルパスを返す
	return outputPath, nil
}

// getImageProperties はsipsコマンドを実行し、画像情報をマップで返します。
// 失敗した場合はnilとエラーを返します。
func imageProperties(filePath string) (map[string]string, error) {
	// sips -g all [ファイルパス] コマンドを構築
	cmd := exec.Command("sips", "-g", "all", filePath)
	
	// コマンドを実行し、標準出力を取得
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("sipsコマンドの実行に失敗しました: %w", err)
	}

	// 正規表現をコンパイル
	// キーと値のペアを抽出するパターン
	re := regexp.MustCompile(`\s*(\S+):\s*(.*)`)

	// 抽出した結果を格納するマップを初期化
	sipsProps := make(map[string]string)

	// 正規表現に一致するすべての部分文字列を検索
	matches := re.FindAllStringSubmatch(string(output), -1)

	for _, match := range matches {
		if len(match) >= 3 {
			key := match[1]
			value := match[2]
			sipsProps[key] = value
		}
	}

	return sipsProps, nil
}



