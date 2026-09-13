import Cocoa
import SwiftUI

if CommandLine.arguments.contains("--snapshot") {
    let appState = AppState.shared
    appState.loadData()
    RunLoop.main.run(until: Date().addingTimeInterval(0.3))

    let view = MainWindowView(appState: appState, onToggleUpperBar: {})
        .frame(width: 780, height: 600)
        .preferredColorScheme(.dark)
    let hosting = NSHostingView(rootView: view)
    hosting.frame = NSRect(x: 0, y: 0, width: 780, height: 600)
    hosting.layoutSubtreeIfNeeded()
    RunLoop.main.run(until: Date().addingTimeInterval(0.2))
    if let rep = hosting.bitmapImageRepForCachingDisplay(in: hosting.bounds) {
        hosting.cacheDisplay(in: hosting.bounds, to: rep)
        if let data = rep.representation(using: .png, properties: [:]) {
            let outPath = CommandLine.arguments.count > 2 ? CommandLine.arguments[2] : "/tmp/gemini_swap_snapshot.png"
            try? data.write(to: URL(fileURLWithPath: outPath))
            print("SNAPSHOT_SAVED: \(outPath)")
        }
    }
    exit(0)
}

let app = NSApplication.shared
let delegate = AppDelegate()
app.delegate = delegate
app.run()
