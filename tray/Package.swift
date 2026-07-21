// swift-tools-version: 6.2
import PackageDescription

let package = Package(
    name: "AvellaTray",
    platforms: [.macOS(.v14)],
    targets: [
        .target(
            name: "AvellaTrayLib",
            path: "Sources/AvellaTrayLib",
            resources: [.process("Resources")]
        ),
        .executableTarget(
            name: "AvellaTray",
            dependencies: ["AvellaTrayLib"],
            path: "Sources/AvellaTray"
        ),
        .testTarget(
            name: "AvellaTrayTests",
            dependencies: ["AvellaTrayLib"],
            path: "Tests/AvellaTrayTests"
        ),
    ]
)
