// swift-tools-version:5.9
// The swift-tools-version declares the minimum version of Swift required to build this package.
// Copyright (c) 2021-2023 Apple Inc. Licensed under MIT License.

import PackageDescription

let package = Package(
    name: "swift-network-quality-server",
    platforms: [
            .tvOS("16.0.internal"),
            .iOS("16.0.internal"),
            .macOS("13.0.internal")
    ],
    dependencies: [
        .package(url: "https://github.com/apple/swift-nio.git", from: "2.48.0"),
        .package(url: "https://github.com/apple/swift-nio-ssl.git", from: "2.23.0"),
        .package(url: "https://github.com/apple/swift-log.git", from: "1.5.2"),
        .package(url: "https://github.com/apple/swift-nio-http2.git", from: "1.24.0"),
        .package(url: "https://github.com/apple/swift-argument-parser", from: "1.2.1"),
    ],
    targets: [
        .executableTarget(
            name: "networkQualityd",
            dependencies: [
                .product(name: "NIO", package: "swift-nio"),
                .product(name: "NIOHTTP1", package: "swift-nio"),
                .product(name: "NIOHTTP2", package: "swift-nio-http2"),
                .product(name: "Logging", package: "swift-log"),
                .product(name: "NIOSSL", package: "swift-nio-ssl"),
                .product(name: "ArgumentParser", package: "swift-argument-parser"),
            ]
        ),
    ]
)
