// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "AvellaTray",
    platforms: [.macOS(.v13)],
    targets: [
        .executableTarget(
            name: "AvellaTray",
            path: "Sources/AvellaTray",
            resources: [.process("Resources")]
        ),
    ]
)
