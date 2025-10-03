//
//  getIcon.swift
//  getIcon
//
//  Created by 癸生川知秀 on 2025/09/14.
//
// swiftc getIcon.swift -o getIcon

// get_icon.swift

import AppKit
import Foundation

let args = CommandLine.arguments
guard args.count > 1 else {
	fputs("Usage: iconfetcher <path> [size]\n", stderr)
	exit(1)
}

let path = args[1]

// デフォルトは 16
let size: CGFloat
if args.count > 2, let s = Double(args[2]) {
	size = CGFloat(s)
} else {
	size = 32
}

let url = URL(fileURLWithPath: path)
let icon = NSWorkspace.shared.icon(forFile: url.path)

// リサイズ描画用の画像を作る
let resized = NSImage(size: NSSize(width: size, height: size))
resized.lockFocus()
icon.draw(in: NSRect(x: 0, y: 0, width: size, height: size),
		  from: .zero,
		  operation: .copy,
		  fraction: 1.0)
resized.unlockFocus()

// PNG へ変換
guard let tiff = resized.tiffRepresentation,
	  let bitmap = NSBitmapImageRep(data: tiff),
	  let pngData = bitmap.representation(using: .png, properties: [:]) else {
	fputs("Failed to get icon\n", stderr)
	exit(1)
}

// Base64 出力
let base64 = pngData.base64EncodedString()
print(base64)
