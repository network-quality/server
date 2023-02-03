//===----------------------------------------------------------------------===//
//
// This source file is part of the SwiftNIO open source project
//
// Copyright (c) 2017-2018 Apple Inc. and the SwiftNIO project authors
// Licensed under Apache License v2.0
//
// See LICENSE.txt for license information
// See CONTRIBUTORS.txt for the list of SwiftNIO project authors
//
// SPDX-License-Identifier: Apache-2.0
//
//===----------------------------------------------------------------------===//

// NOTE: this was adopted and modified from the sample HTTP server code NIOHTTP1Server from the swift-nio codebase

// Copyright (c) 2021-2023 Apple Inc. Licensed under MIT License.

import ArgumentParser
import Foundation
import NIO
import NIOHTTP1
import NIOHTTP2
import NIOSSL
import Logging

let logger = Logger(label: "com.example.networkqualityd.main")

struct ServerOptions: ParsableArguments {
  @Option(help: "IP to bind to")
  var bindHost = "127.0.0.1"

  @Option(help: "http port")
  var httpPort = 4040

  @Option(help: "https port")
  var httpsPort = 4043

  @Option(help: "cert file")
  var certFile: String?

  @Option(help: "key file")
  var keyFile: String?

  @Option(help: "public name")
  var publicName: String?

  @Flag(name: .shortAndLong, inversion: .prefixedNo, help: "allow half closure")
  var allowHalfClosure = true
}

var cfgFileContents = "{}"

final class ErrorHandler: ChannelInboundHandler {
  typealias InboundIn = Never

  func errorCaught(context: ChannelHandlerContext, error: Error) {
    logger.debug("Unhandled HTTP server error: \(error)")
    context.close(mode: .output, promise: nil)
  }
}

// main
let options = ServerOptions.parseOrExit()

let group = MultiThreadedEventLoopGroup(numberOfThreads: System.coreCount)
let threadPool = NIOThreadPool(numberOfThreads: 6)
threadPool.start()

func childChannelInitializer(channel: Channel) -> EventLoopFuture<Void> {
  channel.pipeline.configureHTTPServerPipeline(withErrorHandling: true).flatMap {
    channel.pipeline.addHandler(HTTPHandler())
  }
}

var domainPort:String
if let publicName = options.publicName {
  domainPort = "\(publicName):\(options.httpsPort)"
} else {
  domainPort = "localhost:\(options.httpsPort)"
}

let mc = NetworkQualityConfig(domainPort: domainPort)

let jsonData = try! JSONEncoder().encode(mc)
cfgFileContents = String(data: jsonData, encoding: .utf8)!

let httpsPort = options.httpsPort
guard let keyFile = options.keyFile else {
  logger.error("key file not specified")
  exit(1)
}

guard let certFile = options.certFile else {
  logger.error("cert file not specified")
  exit(1)
}

let certChain = try NIOSSLCertificate.fromPEMFile(certFile).map { NIOSSLCertificateSource.certificate($0) }
let certKey = try NIOSSLPrivateKeySource.privateKey(NIOSSLPrivateKey(file: keyFile, format: .pem))
let tlsConfiguration = TLSConfiguration.forServer(certificateChain: certChain,
                                                  privateKey: certKey,
                                                  applicationProtocols: NIOHTTP2SupportedALPNProtocols)

// Configure the SSL context that is used by all SSL handlers.
let sslContext = try! NIOSSLContext(configuration: tlsConfiguration)
let httpsSocketBootstrap = ServerBootstrap(group: group)
  // Specify backlog and enable SO_REUSEADDR for the server itself
  .serverChannelOption(ChannelOptions.backlog, value: 256)
  .serverChannelOption(ChannelOptions.socketOption(.so_reuseaddr), value: 1)

  // Set the handlers that are applied to the accepted Channels
  .childChannelInitializer { channel in
    // First, we need an SSL handler because HTTP/2 is almost always spoken over TLS.
    channel.pipeline.addHandler(NIOSSLServerHandler(context: sslContext)).flatMap {
      // Right after the SSL handler, we can configure the HTTP/2 pipeline.
      channel.configureHTTP2Pipeline(mode: .server) { (streamChannel) -> EventLoopFuture<Void> in
        // For every HTTP/2 stream that the client opens, transform the
        // HTTP/2 frames to the HTTP/1 messages from the `NIOHTTP1` module.
        streamChannel.pipeline.addHandler(HTTP2FramePayloadToHTTP1ServerCodec()).flatMap { () -> EventLoopFuture<Void> in
          streamChannel.pipeline.addHandler(HTTPHandler())
        }.flatMap { () -> EventLoopFuture<Void> in
          streamChannel.pipeline.addHandler(ErrorHandler())
        }
      }
    }.flatMap { (_: HTTP2StreamMultiplexer) in
      channel.pipeline.addHandler(ErrorHandler())
    }
  }

  .childChannelOption(ChannelOptions.socketOption(.tcp_nodelay), value: 1)
  .childChannelOption(ChannelOptions.socketOption(.so_reuseaddr), value: 1)

  .childChannelOption(ChannelOptions.maxMessagesPerRead, value: 1)
  .childChannelOption(ChannelOptions.allowRemoteHalfClosure, value: options.allowHalfClosure)

let channelHttps = try httpsSocketBootstrap.bind(host: options.bindHost, port: httpsPort).wait()
guard let channelLocalAddress = channelHttps.localAddress else {
  fatalError("Address was unable to bind. Please check that the socket was not closed or that the address family was understood.")
}

logger.info("h2 Listening at \(channelLocalAddress)")

let httpSocketBootstrap = ServerBootstrap(group: group)
  // Specify backlog and enable SO_REUSEADDR for the server itself
  .serverChannelOption(ChannelOptions.backlog, value: 256)
  .serverChannelOption(ChannelOptions.socketOption(.so_reuseaddr), value: 1)

  // Set the handlers that are applied to the accepted Channels
  .childChannelInitializer(childChannelInitializer(channel:))

  // Enable SO_REUSEADDR for the accepted Channels
  .childChannelOption(ChannelOptions.socketOption(.so_reuseaddr), value: 1)
  .childChannelOption(ChannelOptions.maxMessagesPerRead, value: 1)
  .childChannelOption(ChannelOptions.allowRemoteHalfClosure, value: options.allowHalfClosure)

defer {
  try! group.syncShutdownGracefully()
  try! threadPool.syncShutdownGracefully()
}

let channel = try httpSocketBootstrap.bind(host: options.bindHost, port: options.httpPort).wait()

guard let channelLocalAddress = channel.localAddress else {
  fatalError("Address was unable to bind. Please check that the socket was not closed or that the address family was understood.")
}

logger.info("http Listening at \(channelLocalAddress)")
logger.info("Network Quality URL: https://\(domainPort)/config")

// This will never unblock as we don't close the ServerChannel
try channel.closeFuture.wait()

logger.info("Server closed")
