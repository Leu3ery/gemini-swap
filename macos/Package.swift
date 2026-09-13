// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "GeminiSwap",
    platforms: [
        .macOS(.v13)
    ],
    products: [
        .executable(
            name: "GeminiSwap",
            targets: ["GeminiSwap"]
        )
    ],
    targets: [
        .executableTarget(
            name: "GeminiSwap",
            path: "Sources/GeminiSwap"
        )
    ]
)
