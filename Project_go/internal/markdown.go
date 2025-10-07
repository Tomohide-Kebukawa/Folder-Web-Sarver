// Functions/markdown.go:画像ビューアハンドラ:Functions/image.go

package internal

import (
	"html/template"
	"log"
	"fmt"
	"net/http"
	"os"
	"io"
	"path/filepath"

	"strings"
//	"io/ioutil" //MarkdownテキストをHTMLテキスト化用
	"github.com/gomarkdown/markdown"	//MarkdownテキストをHTMLテキストに変換するライブラリ
	
	"golang.org/x/text/encoding/unicode"	//BOM対応
	"golang.org/x/text/transform"			//BOM対応
)


// HandleMarkdownRequestは画像ビューアのHTMLを返します。
func HandleMarkdownRequest(resolvedFolders map[string]string, config *ServerConfig, markdownTmpl *template.Template, err404Tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

			_, err := os.Stat(fullPath)
			if err == nil {
				// MDファイルが存在する場合、テンプレートを返す

				// 1. ファイルの読み込み
//				mdBytes, err := ioutil.ReadFile(fullPath)
				mdBytes, err := readWithBOMOverride(fullPath)
				if err != nil {
					// ファイルが見つからない、または読み込めない場合
					log.Printf("Markdown: ファイルの読み込みに失敗しました: '%s' %v", fullPath, err)
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.WriteHeader(http.StatusInternalServerError)
					err404Tmpl.Execute(w, NotFoundData{WS_Link: r.URL.Path})
					return
				}

				// 2. MarkdownをHTMLに変換
				log.Printf("Markdown: MDのHTML化: '%s'", fullPath)
				htmlContent := MarkdownToHTML(string(mdBytes))		

				// 2. 変換後のHTML文字列を template.HTML 型にキャスト
				safeHTML := template.HTML(htmlContent) 
				
				// WS_BaseURLはURLエンコードされたフォルダパスを返す
				parentURL := filepath.Dir(r.URL.Path) + "/"

				markdownData := MarkdownData{
					WS_Title:			template.URL(filepath.Base(fullPath)),
					WS_Link:			template.URL(r.URL.Path),
					WS_BaseURL:			template.URL(parentURL),
					WS_Content:			safeHTML,
				}

				// 3. レスポンスとしてクライアントに送り返す			
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				if err := markdownTmpl.Execute(w, markdownData); err != nil {
					log.Printf("Markdown: テンプレートの実行に失敗しました: %v", err)
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.WriteHeader(http.StatusInternalServerError)
					err404Tmpl.Execute(w, NotFoundData{WS_Link: r.URL.Path})
				}
				return
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		err404Tmpl.Execute(w, NotFoundData{WS_Link: r.URL.Path})
	}
}


// MarkdownToHTML は、MarkdownテキストをHTMLテキストに変換します。
func MarkdownToHTML(mdText string) string {
	// シンプルな変換を実行
	htmlBytes := markdown.ToHTML([]byte(mdText), nil, nil)
	return string(htmlBytes)
}

// readWithBOMOverride は、BOMを自動で検出し、BOMがあれば除去しつつ
// ファイルの内容をUTF-8文字列として読み込みます。
// UTF-16などの他のエンコーディングも自動でUTF-8に変換されます。
func readWithBOMOverride(filePath string) (string, error) {
	// 1. ファイルを開く
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("ファイルのオープンに失敗しました: %w", err)
	}
	defer f.Close()

	// 2. BOMOverrideとNewReaderを使用してReaderを構築する
	// Decoderでエンコーディングを認識し、BOMを除去し、UTF-8に変換します。
	// unicode.BOMOverride(unicode.UTF8.NewDecoder())
	// - BOMがあれば、BOMを解釈して対応するエンコーディング(UTF-8, UTF-16LE/BE)のDecoderを適用
	// - BOMがなければ、引数で指定したDecoder (この場合はUTF-8)を適用
	decoder := unicode.BOMOverride(unicode.UTF8.NewDecoder())
	
	// transform.NewReaderは、指定されたDecoderに従ってデータを変換しながら読み込むReaderを返します。
	reader := transform.NewReader(f, decoder)

	// 3. 変換された内容をすべて読み込む
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("データの読み込みに失敗しました: %w", err)
	}

	// 4. バイト列をUTF-8文字列に変換して返す
	return string(data), nil
}


