import Foundation

// 引数チェック
guard CommandLine.arguments.count == 2 else {
    print("Usage: resolve-alias <alias-file>")
    exit(1)
}

let aliasPath = CommandLine.arguments[1]
let aliasURL = URL(fileURLWithPath: aliasPath)

do {
    // エイリアスファイルのデータを取得
    let bookmarkData = try URL.bookmarkData(withContentsOf: aliasURL)

    var isStale = false
    // エイリアスを解決して実体の URL を取得
    let resolvedURL = try URL(resolvingBookmarkData: bookmarkData,
                              options: [.withoutUI, .withoutMounting],
                              relativeTo: nil,
                              bookmarkDataIsStale: &isStale)

    print(resolvedURL.path)

} catch {
    fputs("Error: \(error)\n", stderr)
    exit(1)
}
