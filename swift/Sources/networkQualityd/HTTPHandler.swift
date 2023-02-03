//
//  HTTPHandler.swift
//  networkQualityd
//
//  Copyright (c) 2021-2023 Apple Inc. Licensed under MIT License.

import ArgumentParser
import Foundation
import NIO
import NIOHTTP1
import NIOHTTP2
import NIOSSL

private extension String {
	func chopPrefix(_ prefix: String) -> String? {
		if unicodeScalars.starts(with: prefix.unicodeScalars) {
			return String(self[index(startIndex, offsetBy: prefix.count)...])
		} else {
			return nil
		}
	}
}

private func httpResponseHead(request: HTTPRequestHead, status: HTTPResponseStatus, headers: HTTPHeaders = HTTPHeaders()) -> HTTPResponseHead {
	var head = HTTPResponseHead(version: request.version, status: status, headers: headers)
	let connectionHeaders: [String] = head.headers[canonicalForm: "connection"].map { $0.lowercased() }

	if !connectionHeaders.contains("keep-alive"), !connectionHeaders.contains("close") {
		// the user hasn't pre-set either 'keep-alive' or 'close', so we might need to add headers

		switch (request.isKeepAlive, request.version.major, request.version.minor) {
		case (true, 1, 0):
			// HTTP/1.0 and the request has 'Connection: keep-alive', we should mirror that
			head.headers.add(name: "Connection", value: "keep-alive")
		case (false, 1, let n) where n >= 1:
			// HTTP/1.1 (or treated as such) and the request has 'Connection: close', we should mirror that
			head.headers.add(name: "Connection", value: "close")
		default:
			// we should match the default or are dealing with some HTTP that we don't support, let's leave as is
			()
		}
	}
	return head
}

final class HTTPHandler: ChannelInboundHandler {

	public typealias InboundIn = HTTPServerRequestPart
	public typealias OutboundOut = HTTPServerResponsePart

	private enum State {
		case idle
		case waitingForRequestBody
		case sendingResponse

		mutating func requestReceived() {
			precondition(self == .idle, "Invalid state for request received: \(self)")
			self = .waitingForRequestBody
		}

		mutating func requestComplete() {
			precondition(self == .waitingForRequestBody, "Invalid state for request complete: \(self)")
			self = .sendingResponse
		}

		mutating func responseComplete() {
			precondition(self == .sendingResponse, "Invalid state for response complete: \(self)")
			self = .idle
		}
	}

	private var salty = Salty()

	private var buffer: ByteBuffer!
	private var keepAlive = false
	private var state = State.idle
	private var failed = false

	private var infoSavedRequestHead: HTTPRequestHead?
	private var started: Date?
	private var lastInterval: Date?
	private var totalBytes: Int = 0
	private var byteAccumulator: Int = 0

	private var saltySize: Int = 0

	private var handler: ((ChannelHandlerContext, HTTPServerRequestPart) -> Void)?
	private var handlerFuture: EventLoopFuture<Void>?
	private let defaultResponse = "netqualid\r\n"

	func handleUpload(context: ChannelHandlerContext, request: HTTPServerRequestPart) {
		switch request {
		case let .head(request):
			state.requestReceived()

			keepAlive = request.isKeepAlive
			infoSavedRequestHead = request

			if request.method != .POST {
				context.writeAndFlush(wrapOutboundOut(.head(httpResponseHead(request: request, status: .methodNotAllowed))), promise: nil)
				failed = true
				return
			}

			started = Date()
			lastInterval = Date()

			context.writeAndFlush(wrapOutboundOut(.head(httpResponseHead(request: request, status: .continue))), promise: nil)
		case let .body(buffer: buf):
			totalBytes += buf.readableBytes
			let now = Date()
			let interval = now.timeIntervalSince(lastInterval!)
			if interval < 1 {
				byteAccumulator += buf.readableBytes
				return
			}

		case .end:
			state.requestComplete()
			if infoSavedRequestHead?.method != .POST {
				completeResponse(context, trailers: nil, promise: nil)
				return
			}

			let now = Date()
			let interval = now.timeIntervalSince(lastInterval!)

			let millis = interval * 1000

			let totalTime = now.timeIntervalSince(started!)
			let millisTotal = totalTime * 1000

			let throughput = (Double(byteAccumulator * 8) / interval) / 1024.0 / 1024.0

			let string = "HTTP/1.1 200 OK\nInterval: \(millis)\nTotal-Bytes: \(totalBytes)\nThroughput: \(throughput)\nTotal-Time: \(millisTotal)\nContent-Length: 0\nConnection: close\n\n"
			var buf = context.channel.allocator.buffer(capacity: string.utf8.count)
			buf.writeString(string)
			context.writeAndFlush(wrapOutboundOut(.body(.byteBuffer(buf))), promise: nil)

			completeResponse(context, trailers: nil, promise: nil)
		}
	}

	func handleUploadWith100(context: ChannelHandlerContext, request: HTTPServerRequestPart) {
		switch request {
		case let .head(request):
			state.requestReceived()

			keepAlive = request.isKeepAlive
			infoSavedRequestHead = request

			if request.method != .POST {
				context.writeAndFlush(wrapOutboundOut(.head(httpResponseHead(request: request, status: .methodNotAllowed))), promise: nil)
				failed = true

				return
			}

			started = Date()
			lastInterval = Date()

			context.writeAndFlush(wrapOutboundOut(.head(httpResponseHead(request: request, status: .continue))), promise: nil)
		case let .body(buffer: buf):
			totalBytes += buf.readableBytes
			let now = Date()
			let interval = now.timeIntervalSince(lastInterval!)
			if interval < 1 {
				byteAccumulator += buf.readableBytes
				return
			}

			let throughput = (Double(byteAccumulator * 8) / interval) / 1024.0 / 1024.0
			byteAccumulator = 0
			lastInterval = now
			let millis = interval * 1000
			let string = "HTTP/1.1 100 Continue\nInterval: \(millis)\nTotal-Bytes: \(totalBytes)\nThroughput: \(throughput)\n\n"

			var buf = context.channel.allocator.buffer(capacity: string.utf8.count)
			buf.writeString(string)
			context.writeAndFlush(wrapOutboundOut(.body(.byteBuffer(buf))), promise: nil)
		case .end:
			state.requestComplete()
			if infoSavedRequestHead?.method != .POST {
				completeResponse(context, trailers: nil, promise: nil)
				return
			}

			let now = Date()
			let interval = now.timeIntervalSince(lastInterval!)

			let millis = interval * 1000

			let totalTime = now.timeIntervalSince(started!)
			let millisTotal = totalTime * 1000

			let throughput = (Double(byteAccumulator * 8) / interval) / 1024.0 / 1024.0

			let string = "HTTP/1.1 200 OK\nInterval: \(millis)\nTotal-Bytes: \(totalBytes)\nThroughput: \(throughput)\nTotal-Time: \(millisTotal)\nContent-Length: 0\nConnection: close\n\n"
			var buf = context.channel.allocator.buffer(capacity: string.utf8.count)
			buf.writeString(string)
			context.writeAndFlush(wrapOutboundOut(.body(.byteBuffer(buf))), promise: nil)

			completeResponse(context, trailers: nil, promise: nil)
		}
	}

	func handleSmall(context: ChannelHandlerContext, request: HTTPServerRequestPart) {
		handleSalty(context: context, request: request, size: 1)
	}

	func handleLarge(context: ChannelHandlerContext, request: HTTPServerRequestPart) {
		handleSalty(context: context, request: request, size:8*1024*1024*1024)
	}

	let blockSize = 1024 * 1024 * 64

	func handleSalty(context: ChannelHandlerContext, request: HTTPServerRequestPart) {
		let origRequest = request
		switch request {
		case let .head(request):
			keepAlive = request.isKeepAlive
			if request.method != .GET {
				self.state.requestReceived()

				context.writeAndFlush(wrapOutboundOut(.head(httpResponseHead(request: request, status: .methodNotAllowed))), promise: nil)
				failed = true
				return
			}

			if let fileName = request.uri.chopPrefix("/salty/") {
				guard let size = salty.saltySizes[fileName] else {
					logger.error("bad size request \(fileName)")

					self.state.requestReceived()
					context.writeAndFlush(wrapOutboundOut(.head(httpResponseHead(request: request, status: .notFound))), promise: nil)
					failed = true
					return
				}

				return self.handleSalty(context: context, request: origRequest, size: size)
			}
		default:
			self.state.requestComplete()
			if saltySize == 0 || failed {
				completeResponse(context, trailers: nil, promise: nil)
			}
			break
		}
	}

	func handleSalty(context: ChannelHandlerContext, request: HTTPServerRequestPart, size: Int) {
		saltySize = size
		switch request {
		case let .head(request):
			state.requestReceived()
			if request.method != .GET {
				context.writeAndFlush(wrapOutboundOut(.head(httpResponseHead(request: request, status: .methodNotAllowed))), promise: nil)
				failed = true
				return
			}

			func doNext() {
				self.buffer.clear()
				if (saltySize == totalBytes) {
					self.completeResponse(context, trailers: nil, promise: nil)
					return
				}

				var nextBlock = blockSize
				if (saltySize < blockSize) {
					nextBlock = saltySize
				}

				self.buffer.writeRepeatingByte(UInt8(114), count: nextBlock)

				totalBytes += nextBlock

				context.writeAndFlush(self.wrapOutboundOut(.body(.byteBuffer(self.buffer)))).map {
					context.eventLoop.scheduleTask(in: .milliseconds(0), doNext)
				}.whenFailure { (_: Error) in
					self.completeResponse(context, trailers: nil, promise: nil)
				}
			}

			var myHead = httpResponseHead(request: request, status: .ok)
			myHead.headers.add(name: "Proxy-Cache-Control", value: "max-age=604800, public")
			myHead.headers.add(name: "Content-Length", value: String(saltySize))
			myHead.headers.add(name: "Cache-Control", value: "no-store, must-revalidate, private, max-age=0")
			myHead.headers.add(name: "Content-Type", value: "application/octet-stream")

			context.writeAndFlush(wrapOutboundOut(.head(myHead)), promise: nil)

			doNext()

		case .end:
			self.state.requestComplete()
			if saltySize == 0 || failed {
				completeResponse(context, trailers: nil, promise: nil)
			}
		default:
			break
		}
	}

	func handleConfig(context: ChannelHandlerContext, request: HTTPServerRequestPart) {
		switch request {
		case let .head(request):
			keepAlive = request.isKeepAlive
			state.requestReceived()
			if request.method != .GET {
				context.writeAndFlush(wrapOutboundOut(.head(httpResponseHead(request: request, status: .methodNotAllowed))), promise: nil)
				return
			}

			var myHead = httpResponseHead(request: request, status: .ok)
			myHead.headers.add(name: "Content-Type", value: "application/json")
			context.writeAndFlush(wrapOutboundOut(.head(myHead)), promise: nil)
		case .body(buffer: _):
			()
		case .end:
			state.requestComplete()

			var buf = context.channel.allocator.buffer(capacity: cfgFileContents.utf8.count)
			buf.writeString(cfgFileContents)
			context.writeAndFlush(wrapOutboundOut(.body(.byteBuffer(buf))), promise: nil)
			completeResponse(context, trailers: nil, promise: nil)
		}
	}

	private func completeResponse(_ context: ChannelHandlerContext, trailers: HTTPHeaders?, promise: EventLoopPromise<Void>?) {
		state.responseComplete()

		let promise = keepAlive ? promise : (promise ?? context.eventLoop.makePromise())
		if !keepAlive {
			promise!.futureResult.whenComplete { (_: Result<Void, Error>) in context.close(promise: nil) }
		}
		handler = nil

		context.writeAndFlush(wrapOutboundOut(.end(trailers)), promise: promise)
	}

	func channelRead(context: ChannelHandlerContext, data: NIOAny) {
		let reqPart = unwrapInboundIn(data)
		if let handler = self.handler {
			handler(context, reqPart)
			return
		}

		switch reqPart {
		case let .head(request):
			if request.uri.unicodeScalars.starts(with: "/slurp".unicodeScalars) {
				handler = handleUpload
				handler!(context, reqPart)
				return
			} else if request.uri.unicodeScalars.starts(with: "/small".unicodeScalars) {
				handler = handleSmall
				handler!(context, reqPart)
				return
			} else if request.uri.unicodeScalars.starts(with: "/large".unicodeScalars) {
				handler = handleLarge
				handler!(context, reqPart)
				return
			} else if request.uri.unicodeScalars.starts(with: "/config".unicodeScalars) {
				handler = handleConfig
				handler!(context, reqPart)
				return
			} else if request.uri.unicodeScalars.starts(with: "/.well-known/nq".unicodeScalars) {
				handler = handleConfig
				handler!(context, reqPart)
				return
			}

			keepAlive = request.isKeepAlive
			state.requestReceived()

		  	if request.method != .GET {
				context.writeAndFlush(wrapOutboundOut(.head(httpResponseHead(request: request, status: .methodNotAllowed))), promise: nil)
				return
			}

			var responseHead = httpResponseHead(request: request, status: HTTPResponseStatus.ok)
			buffer.clear()
			buffer.writeString(defaultResponse)
			responseHead.headers.add(name: "content-length", value: "\(buffer!.readableBytes)")

			let response = HTTPServerResponsePart.head(responseHead)
			context.write(wrapOutboundOut(response), promise: nil)
		case .body:
			break
		case .end:
			state.requestComplete()
			let content = HTTPServerResponsePart.body(.byteBuffer(buffer!.slice()))
			context.write(wrapOutboundOut(content), promise: nil)
			completeResponse(context, trailers: nil, promise: nil)
		}
	}

	func channelReadComplete(context: ChannelHandlerContext) {
		context.flush()
	}

	func handlerAdded(context: ChannelHandlerContext) {
		buffer = context.channel.allocator.buffer(capacity: 0)
	}

	func userInboundEventTriggered(context: ChannelHandlerContext, event: Any) {
		switch event {
		case let evt as ChannelEvent where evt == ChannelEvent.inputClosed:
			// The remote peer half-closed the channel. At this time, any
			// outstanding response will now get the channel closed, and
			// if we are idle or waiting for a request body to finish we
			// will close the channel immediately.
			switch state {
			case .idle, .waitingForRequestBody:
				context.close(promise: nil)
			case .sendingResponse:
				keepAlive = false
			}
		default:
			context.fireUserInboundEventTriggered(event)
		}
	}
}
