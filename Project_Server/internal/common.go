// Functions/common.go:共通関数と構造体:Functions/common.go
//
// 変数定義はここでまとめてする
//

package internal

import (
	"html/template"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"mime"
)

// ServerConfigはsettings.jsonの構造を定義します。
type ServerConfig struct {
	Config struct {
		Server struct {
			Port int `json:"port"`
		} `json:"server"`
		Templates map[string]string `json:"templates"`
		Temporary string
	} `json:"config"`
	Folders []string `json:"folders"`
	Ignores []string `json:"ignores"`
}

// NotFoundDataは404テンプレートに渡されるデータを定義します。
type NotFoundData struct {
	WS_Link string
}

// WS_Object はフォルダリスト内の1つの項目を表す
type WS_FileEntry struct {
	WS_Name			string
	WS_Link			string
	WS_Size			string
	WS_LastMod			string
	WS_IsDirectory	bool
	WS_IsMovie		bool
	WS_IsImage		bool
	WS_IconPath		template.URL
}

// FolderDataはフォルダテンプレートに渡されるデータを定義します。
type FolderData struct {
	WS_Title		string
	WS_Link			string
	WS_ParentPath	string
	WS_Objects		[]WS_FileEntry
}

// ImageDataは画像表示テンプレートに渡されるデータを定義します。
type ImageData struct {
	WS_Title		template.URL
	WS_Link			template.URL
	WS_BaseURL		template.URL
	WS_CurrentIndex	int
	WS_ImagePaths	[]string
}

// VideoTemplateDataは動画表示テンプレートに渡されるデータを定義します。
type VideoTemplateData struct {
	WS_Title	string
	WS_Link		string
	WS_BaseURL	template.URL
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

// IsMovieFileはファイルが動画ファイルであるかどうかをチェックします。
func IsMovieFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	// 一般的に使用される動画拡張子をチェック
	return ext == ".mkv" || ext == ".mov" || ext == ".avi" || ext == ".webm" || ext == ".mp4" || ext == ".wmv" || ext == ".flv"
}

// isImageFileはファイルが画像ファイルであるかどうかをチェックします。
func isImageFile(path string) bool {
	mimeType := mime.TypeByExtension(filepath.Ext(path))
	return strings.HasPrefix(mimeType, "image/")
}
