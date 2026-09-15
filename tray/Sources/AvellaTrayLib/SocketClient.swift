import Foundation
import Network

// MARK: - SocketEvent

/// Events emitted by `SocketClient` as the connection to the daemon changes state.
enum SocketEvent: Sendable {
  case connected
  case disconnected
  case protocolMismatch(daemonVersion: Int)
  case state(AppState)
}

// MARK: - LineBuffer

/// Accumulates raw bytes and splits them into newline-terminated lines.
///
/// Pure value type — no Foundation networking dependency, fully unit-testable.
struct LineBuffer {
  /// Appends new bytes and returns any complete (newline-terminated) lines,
  /// with the trailing newline stripped. Incomplete trailing data is
  /// retained for the next call.
  mutating func append(_ data: Data) -> [Data] {
    pending.append(data)

    var lines = [Data]()
    while let newlineIndex = pending.firstIndex(of: UInt8(ascii: "\n")) {
      let line = Data(pending[pending.startIndex..<newlineIndex])
      lines.append(line)
      pending = Data(pending[pending.index(after: newlineIndex)...])
    }
    return lines
  }

  private var pending = Data()
}

// MARK: - Backoff

/// Exponential backoff sequence starting at 1s, doubling, capped at 10s.
struct Backoff {

  // MARK: Lifecycle

  init(initial: TimeInterval = 1.0, cap: TimeInterval = 10.0) {
    self.initial = initial
    self.cap = cap
    current = initial
  }

  // MARK: Internal

  private(set) var current: TimeInterval
  let initial: TimeInterval
  let cap: TimeInterval

  /// Returns the current interval, then doubles it (capped) for next time.
  mutating func next() -> TimeInterval {
    let value = current
    current = min(current * 2, cap)
    return value
  }

  mutating func reset() {
    current = initial
  }

}

// MARK: - SocketClient

/// Connects to the Avella daemon over a Unix domain socket using
/// Network.framework, receives state updates, and sends commands.
actor SocketClient {

  // MARK: Lifecycle

  init(socketPath: String = SocketClient.defaultSocketPath()) {
    self.socketPath = socketPath
    var continuation: AsyncStream<SocketEvent>.Continuation!
    events = AsyncStream { cont in
      continuation = cont
    }
    self.continuation = continuation
  }

  // MARK: Internal

  nonisolated let events: AsyncStream<SocketEvent>

  /// Test-only: current backoff interval, to assert it advances exactly
  /// once per disconnect (see the double-callback bug this guards against).
  var currentBackoffForTesting: TimeInterval {
    backoff.current
  }

  static func defaultSocketPath() -> String {
    let home = FileManager.default.homeDirectoryForCurrentUser
    return home.appendingPathComponent(".cache/avella/avella.sock").path
  }

  func start() {
    stopped = false
    connect()
  }

  func stop() {
    stopped = true
    reconnectTask?.cancel()
    reconnectTask = nil
    closeConnection(emitDisconnected: true)
  }

  /// Sends a command to the daemon and awaits the flush to the socket.
  /// Returns false when there is no ready connection, encoding fails, or
  /// the write errors — the command was dropped.
  @discardableResult
  func send(_ command: DaemonCommand) async -> Bool {
    guard let connection, ready else { return false }
    let cmd = ClientCommand(command)
    guard var data = try? JSONEncoder().encode(cmd) else { return false }
    data.append(UInt8(ascii: "\n"))
    return await withCheckedContinuation { cont in
      connection.send(content: data, completion: .contentProcessed { error in
        cont.resume(returning: error == nil)
      })
    }
  }

  /// Test-only: replays the two callbacks a single disconnect produces —
  /// the stateUpdateHandler firing .failed and the pending receive
  /// completing — both carrying the generation captured when they were
  /// registered, exactly as Network.framework delivers them. Deterministic
  /// substitute for killing a real connection, whose event delivery timing
  /// is not reliable enough to assert on.
  func simulateDisconnectCallbacksForTesting() {
    let gen = generation
    handleStateUpdate(.failed(.posix(.ECONNRESET)), generation: gen)
    handleReceive(data: nil, isComplete: true, error: nil, generation: gen)
  }

  // MARK: Private

  private let socketPath: String
  private var connection: NWConnection?
  private var lineBuffer = LineBuffer()
  private var backoff = Backoff()
  private var stopped = false
  private var handshakeDone = false
  private var ready = false
  private var reconnectTask: Task<Void, Never>?

  /// Identity of the current connection. Bumped on every connect and close
  /// so callbacks from a superseded connection are dropped: a single
  /// disconnect fires both the stateUpdateHandler and the pending receive
  /// completion, and without this guard the second one would advance the
  /// backoff again (or cancel a freshly established connection).
  private var generation = 0

  private let continuation: AsyncStream<SocketEvent>.Continuation

  private func connect() {
    guard !stopped else { return }

    handshakeDone = false
    lineBuffer = LineBuffer()

    generation += 1
    let gen = generation

    let endpoint = NWEndpoint.unix(path: socketPath)
    let params = NWParameters.tcp
    let newConnection = NWConnection(to: endpoint, using: params)
    connection = newConnection

    newConnection.stateUpdateHandler = { [weak self] state in
      guard let self else { return }
      Task { await self.handleStateUpdate(state, generation: gen) }
    }

    newConnection.start(queue: .global(qos: .utility))
  }

  private func handleStateUpdate(_ state: NWConnection.State, generation gen: Int) {
    guard gen == generation else { return }
    switch state {
    case .ready:
      backoff.reset()
      ready = true
      continuation.yield(.connected)
      receiveNext(generation: gen)

    case .waiting,
         .failed,
         .cancelled:
      closeConnection(emitDisconnected: true)
      scheduleReconnect()

    default:
      break
    }
  }

  private func receiveNext(generation gen: Int) {
    guard let connection else { return }
    connection.receive(minimumIncompleteLength: 1, maximumLength: 65536) { [weak self] data, _, isComplete, error in
      guard let self else { return }
      Task {
        await self.handleReceive(data: data, isComplete: isComplete, error: error, generation: gen)
      }
    }
  }

  private func handleReceive(data: Data?, isComplete: Bool, error: NWError?, generation gen: Int) {
    guard gen == generation, !stopped else { return }

    if let data, !data.isEmpty {
      let lines = lineBuffer.append(data)
      for line in lines {
        processLine(line)
      }
    }

    if isComplete || error != nil {
      closeConnection(emitDisconnected: true)
      scheduleReconnect()
      return
    }

    // Only continue receiving if we haven't been stopped/closed by a
    // protocol mismatch mid-loop.
    guard connection != nil else { return }
    receiveNext(generation: gen)
  }

  private func processLine(_ line: Data) {
    guard !line.isEmpty else { return }
    guard let message = try? JSONDecoder.avella().decode(ServerMessage.self, from: line) else {
      return
    }

    switch message {
    case .hello(let hello):
      if hello.protocolVersion != supportedProtocolVersion {
        let version = hello.protocolVersion
        continuation.yield(.protocolMismatch(daemonVersion: version))
        // Incompatible — disconnect permanently, no reconnect.
        stopped = true
        closeConnection(emitDisconnected: false)
        return
      }
      handshakeDone = true

    case .state(let appState):
      guard handshakeDone else { return }
      continuation.yield(.state(appState))

    case .unknown:
      break
    }
  }

  private func closeConnection(emitDisconnected: Bool) {
    // Bump even when no connection is open so any straggler callback
    // (stale generation) can never trigger a second reconnect.
    generation += 1
    ready = false
    guard let connection else { return }
    connection.stateUpdateHandler = nil
    connection.cancel()
    self.connection = nil
    if emitDisconnected {
      continuation.yield(.disconnected)
    }
  }

  private func scheduleReconnect() {
    guard !stopped else { return }
    reconnectTask?.cancel()
    let interval = backoff.next()
    reconnectTask = Task { [weak self] in
      try? await Task.sleep(nanoseconds: UInt64(interval * 1_000_000_000))
      guard let self, !Task.isCancelled else { return }
      await connect()
    }
  }

}
