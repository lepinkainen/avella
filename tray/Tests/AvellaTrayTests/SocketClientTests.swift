import Foundation
import Network
import XCTest
@testable import AvellaTrayLib

final class SocketClientTests: XCTestCase {

    // MARK: - Helpers

    /// Short unix socket path in /tmp to stay under the 104-byte sun_path limit.
    private func makeTempSocketPath() -> String {
        "/tmp/av\(Int.random(in: 0..<1_000_000)).sock"
    }

    /// Minimal end-to-end server: accepts one connection, writes hello + state lines.
    private final class FakeDaemon: @unchecked Sendable {
        let listener: NWListener
        let queue = DispatchQueue(label: "FakeDaemon")
        private(set) var connectionCount = 0
        private var currentConnection: NWConnection?
        private var receivedLines: [String] = []
        private var lineBuffer = LineBuffer()
        private let lock = NSLock()
        var onNewConnection: (() -> Void)?

        init(socketPath: String, helloProtocolVersion: Int) throws {
            let params = NWParameters.tcp
            params.requiredLocalEndpoint = .unix(path: socketPath)
            listener = try NWListener(using: params)
            listener.newConnectionHandler = { [weak self] connection in
                guard let self else { return }
                self.lock.lock()
                self.connectionCount += 1
                self.currentConnection = connection
                self.lock.unlock()
                connection.start(queue: self.queue)
                self.sendHandshake(over: connection, helloProtocolVersion: helloProtocolVersion)
                self.receiveLoop(over: connection)
                self.onNewConnection?()
            }
        }

        func start() {
            listener.start(queue: queue)
        }

        func stop() {
            listener.cancel()
        }

        var currentConnectionCount: Int {
            lock.lock()
            defer { lock.unlock() }
            return connectionCount
        }

        /// Lines received from the client so far, split on newlines.
        var linesReceivedFromClient: [String] {
            lock.lock()
            defer { lock.unlock() }
            return receivedLines
        }

        private func sendHandshake(over connection: NWConnection, helloProtocolVersion: Int) {
            let hello = "{\"type\":\"hello\",\"data\":{\"protocol_version\":\(helloProtocolVersion)}}\n"
            let state = """
            {"type":"state","data":{"status":"watching","processed":3,"dry_run":false,"config_path":"/tmp/c.yaml","rules":[],"version":"9.9.9","recent_files":[]}}\n
            """
            connection.send(content: Data(hello.utf8), completion: .contentProcessed { _ in
                connection.send(content: Data(state.utf8), completion: .contentProcessed { _ in })
            })
        }

        private func receiveLoop(over connection: NWConnection) {
            connection.receive(minimumIncompleteLength: 1, maximumLength: 65536) { [weak self] data, _, isComplete, error in
                guard let self else { return }
                if let data, !data.isEmpty {
                    self.lock.lock()
                    let lines = self.lineBuffer.append(data)
                    self.receivedLines.append(contentsOf: lines.map { String(decoding: $0, as: UTF8.self) })
                    self.lock.unlock()
                }
                if isComplete || error != nil { return }
                self.receiveLoop(over: connection)
            }
        }
    }

    // MARK: - Unix socket path

    func testDefaultSocketPath() {
        let path = SocketClient.defaultSocketPath()
        XCTAssertTrue(path.hasSuffix(".cache/avella/avella.sock"))
    }

    // MARK: - Real end-to-end socket test

    func testConnectAndReceiveState() async throws {
        let socketPath = makeTempSocketPath()
        defer { try? FileManager.default.removeItem(atPath: socketPath) }

        let daemon = try FakeDaemon(socketPath: socketPath, helloProtocolVersion: supportedProtocolVersion)
        daemon.start()
        defer { daemon.stop() }

        let client = SocketClient(socketPath: socketPath)
        await client.start()
        defer { Task { await client.stop() } }

        var sawConnected = false
        var receivedState: AppState?

        let deadline = Date().addingTimeInterval(5)
        for await event in client.events {
            switch event {
            case .connected:
                sawConnected = true
            case .state(let state):
                receivedState = state
            default:
                break
            }
            if receivedState != nil { break }
            if Date() > deadline { break }
        }

        XCTAssertTrue(sawConnected)
        XCTAssertEqual(receivedState?.status, "watching")
        XCTAssertEqual(receivedState?.processed, 3)
        XCTAssertEqual(receivedState?.version, "9.9.9")
    }

    func testProtocolMismatchStopsWithoutReconnect() async throws {
        let socketPath = makeTempSocketPath()
        defer { try? FileManager.default.removeItem(atPath: socketPath) }

        let daemon = try FakeDaemon(socketPath: socketPath, helloProtocolVersion: supportedProtocolVersion + 99)
        daemon.start()
        defer { daemon.stop() }

        let client = SocketClient(socketPath: socketPath)
        await client.start()
        defer { Task { await client.stop() } }

        var mismatchVersion: Int?
        let deadline = Date().addingTimeInterval(5)
        for await event in client.events {
            if case .protocolMismatch(let version) = event {
                mismatchVersion = version
                break
            }
            if Date() > deadline { break }
        }

        XCTAssertEqual(mismatchVersion, supportedProtocolVersion + 99)

        // Give the client a moment to (incorrectly) reconnect, if it were going to.
        try await Task.sleep(nanoseconds: 500_000_000)
        XCTAssertEqual(daemon.currentConnectionCount, 1, "client must not reconnect after a protocol mismatch")
    }

    func testReconnectsWhenDaemonStartsLate() async throws {
        let socketPath = makeTempSocketPath()
        defer { try? FileManager.default.removeItem(atPath: socketPath) }

        // No listener yet: NWConnection should enter `.waiting`, which must be
        // treated like `.failed` so the client fails fast and backs off,
        // rather than sitting idle forever waiting for a path-change event
        // that will never come for a missing UDS file.
        let client = SocketClient(socketPath: socketPath)
        await client.start()
        defer { Task { await client.stop() } }

        // Let the client observe `.waiting` and enter its backoff/reconnect loop.
        try await Task.sleep(nanoseconds: 1_500_000_000)

        let daemon = try FakeDaemon(socketPath: socketPath, helloProtocolVersion: supportedProtocolVersion)
        daemon.start()
        defer { daemon.stop() }

        var receivedState: AppState?
        let deadline = Date().addingTimeInterval(8)
        for await event in client.events {
            if case .state(let state) = event {
                receivedState = state
                break
            }
            if Date() > deadline { break }
        }

        XCTAssertNotNil(receivedState, "client should reconnect once the daemon becomes available")
        XCTAssertEqual(receivedState?.status, "watching")
    }

    // MARK: - Generation guard against duplicate disconnect callbacks

    func testBackoffAdvancesOnlyOnceOnDisconnect() async throws {
        let socketPath = makeTempSocketPath()
        defer { try? FileManager.default.removeItem(atPath: socketPath) }

        let daemon = try FakeDaemon(socketPath: socketPath, helloProtocolVersion: supportedProtocolVersion)
        daemon.start()
        defer { daemon.stop() }

        let client = SocketClient(socketPath: socketPath)
        await client.start()
        defer { Task { await client.stop() } }

        // Wait for a successful connect, which resets backoff to its initial value (1.0).
        let connectDeadline = Date().addingTimeInterval(5)
        for await event in client.events {
            if case .connected = event { break }
            if Date() > connectDeadline { break }
        }
        let initialBackoff = await client.currentBackoffForTesting
        XCTAssertEqual(initialBackoff, 1.0)

        // Replay the two callbacks a single daemon disconnect produces.
        // (Killing the real connection is not usable here: NW's delivery of
        // the resulting events is too timing-dependent to assert on.)
        await client.simulateDisconnectCallbacksForTesting()

        // A single disconnect must advance backoff exactly once: 1.0 -> 2.0.
        // Without the generation guard both callbacks call backoff.next(),
        // giving 4.0. The actor serializes the replay before this read, so
        // no sleep is needed.
        let backoff = await client.currentBackoffForTesting
        XCTAssertEqual(backoff, 2.0, "backoff must advance exactly once per disconnect")
    }

    // MARK: - send

    func testSendWithoutConnectionReturnsFalse() async {
        let client = SocketClient(socketPath: makeTempSocketPath())
        let sent = await client.send(.toggleDryRun)
        XCTAssertFalse(sent, "send must report failure when there is no connection")
    }

    func testSendDeliversCommandToDaemon() async throws {
        let socketPath = makeTempSocketPath()
        defer { try? FileManager.default.removeItem(atPath: socketPath) }

        let daemon = try FakeDaemon(socketPath: socketPath, helloProtocolVersion: supportedProtocolVersion)
        daemon.start()
        defer { daemon.stop() }

        let client = SocketClient(socketPath: socketPath)
        await client.start()
        defer { Task { await client.stop() } }

        let connectDeadline = Date().addingTimeInterval(5)
        for await event in client.events {
            if case .connected = event { break }
            if Date() > connectDeadline { break }
        }

        let sent = await client.send(.toggleDryRun)
        XCTAssertTrue(sent, "send must succeed and await the flush on a ready connection")

        // Poll until the daemon has read the line off the socket.
        let deadline = Date().addingTimeInterval(5)
        while daemon.linesReceivedFromClient.isEmpty, Date() < deadline {
            try await Task.sleep(nanoseconds: 50_000_000)
        }

        let lines = daemon.linesReceivedFromClient
        XCTAssertEqual(lines.count, 1)
        let decoded = try JSONDecoder().decode([String: String].self, from: Data(lines[0].utf8))
        XCTAssertEqual(decoded, ["type": "command", "command": "toggle_dry_run"])
    }
}
