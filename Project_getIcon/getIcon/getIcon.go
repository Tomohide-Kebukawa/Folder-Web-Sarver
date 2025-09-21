// Goからの呼び出しテスト用

package main

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go-run <path>")
		return
	}

	path := os.Args[1]

	// Swift の CLI を呼び出す
	cmd := exec.Command("./getIcon", path)
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Base64 デコード
	data, err := base64.StdEncoding.DecodeString(string(output))
	if err != nil {
		fmt.Println("Base64 decode error:", err)
		return
	}

	// 保存して確認
	err = ioutil.WriteFile("icon.png", data, 0644)
	if err != nil {
		fmt.Println("Write file error:", err)
		return
	}

	fmt.Println("Icon saved as icon.png")
}
