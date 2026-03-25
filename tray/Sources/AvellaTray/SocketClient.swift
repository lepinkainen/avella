import Foundation

/// Connects to the Avella daemon over a Unix domain socket,
/// receives state updates, and sends commands.
final class SocketClient {
    private let socketPath: String
    private var inputStream: InputStream?
    private var outputStream: OutputStream?
    private var readThread: Thread?
    private var reconnectTimer: Timer?
    private var backoff: TimeInterval = 1.0
    private let maxBackoff: TimeInterval = 10.0
    private var stopping = false

    var onStateUpdate: ((AppState) -> Void)?
    var onConnectionChange: ((Bool) -> Void)?

    init() {
        let home = FileManager.default.homeDirectoryForCurrentUser
        self.socketPath = home
            .appendingPathComponent(".cache/avella/avella.sock")
            .path
    }

    func start() {
        stopping = false
        connect()
    }

    func stop() {
        stopping = true
        reconnectTimer?.invalidate()
        reconnectTimer = nil
        disconnect()
    }

    func send(command: String) {
        guard let output = outputStream, output.streamStatus == .open else { return }
        let cmd = ClientCommand(command: command)
        guard var data = try? JSONEncoder().encode(cmd) else { return }
        data.append(contentsOf: [UInt8(ascii: "\n")])
        data.withUnsafeBytes { buf in
            guard let ptr = buf.baseAddress?.assumingMemoryBound(to: UInt8.self) else { return }
            output.write(ptr, maxLength: data.count)
        }
    }

    // MARK: - Private

    private func connect() {
        guard !stopping else { return }

        // Create a Unix domain socket manually.
        let fd = Darwin.socket(AF_UNIX, SOCK_STREAM, 0)
        guard fd >= 0 else {
            scheduleReconnect()
            return
        }

        var addr = sockaddr_un()
        addr.sun_family = sa_family_t(AF_UNIX)
        let pathBytes = socketPath.utf8CString
        guard pathBytes.count <= MemoryLayout.size(ofValue: addr.sun_path) else {
            Darwin.close(fd)
            scheduleReconnect()
            return
        }
        withUnsafeMutablePointer(to: &addr.sun_path) { sunPathPtr in
            sunPathPtr.withMemoryRebound(to: CChar.self, capacity: pathBytes.count) { dest in
                for (i, byte) in pathBytes.enumerated() {
                    dest[i] = byte
                }
            }
        }

        let addrLen = socklen_t(MemoryLayout<sockaddr_un>.size)
        let result = withUnsafePointer(to: &addr) { ptr in
            ptr.withMemoryRebound(to: sockaddr.self, capacity: 1) { sockPtr in
                Darwin.connect(fd, sockPtr, addrLen)
            }
        }

        guard result == 0 else {
            Darwin.close(fd)
            scheduleReconnect()
            return
        }

        // Wrap fd in streams.
        var readStream: Unmanaged<CFReadStream>?
        var writeStream: Unmanaged<CFWriteStream>?
        CFStreamCreatePairWithSocket(kCFAllocatorDefault, Int32(fd), &readStream, &writeStream)

        guard let cfRead = readStream, let cfWrite = writeStream else {
            Darwin.close(fd)
            scheduleReconnect()
            return
        }

        let input = cfRead.takeRetainedValue() as InputStream
        let output = cfWrite.takeRetainedValue() as OutputStream

        // Tell the streams to close the fd when they are closed.
        let closeKey = CFStreamPropertyKey(rawValue: kCFStreamPropertyShouldCloseNativeSocket)
        CFReadStreamSetProperty(input as CFReadStream, closeKey, kCFBooleanTrue)
        CFWriteStreamSetProperty(output as CFWriteStream, closeKey, kCFBooleanTrue)

        input.open()
        output.open()

        self.inputStream = input
        self.outputStream = output
        self.backoff = 1.0

        DispatchQueue.main.async { [weak self] in
            self?.onConnectionChange?(true)
        }

        // Read in a background thread.
        let thread = Thread { [weak self] in
            self?.readLoop(input: input)
        }
        thread.name = "AvellaTray.SocketReader"
        thread.start()
        self.readThread = thread
    }

    private func disconnect() {
        inputStream?.close()
        outputStream?.close()
        inputStream = nil
        outputStream = nil
        readThread?.cancel()
        readThread = nil
        DispatchQueue.main.async { [weak self] in
            self?.onConnectionChange?(false)
        }
    }

    private func readLoop(input: InputStream) {
        let bufferSize = 4096
        var buffer = [UInt8](repeating: 0, count: bufferSize)
        var leftover = Data()

        while !Thread.current.isCancelled && input.streamStatus != .closed {
            guard input.hasBytesAvailable else {
                Thread.sleep(forTimeInterval: 0.05)
                continue
            }

            let bytesRead = input.read(&buffer, maxLength: bufferSize)
            if bytesRead <= 0 {
                break
            }

            leftover.append(contentsOf: buffer[0..<bytesRead])

            // Split on newlines.
            while let newlineIndex = leftover.firstIndex(of: UInt8(ascii: "\n")) {
                let lineData = leftover[leftover.startIndex..<newlineIndex]
                leftover = Data(leftover[leftover.index(after: newlineIndex)...])

                guard !lineData.isEmpty else { continue }
                let decoder = JSONDecoder()
                if let msg = try? decoder.decode(ServerMessage.self, from: lineData),
                   msg.type == "state",
                   let appState = msg.data {
                    DispatchQueue.main.async { [weak self] in
                        self?.onStateUpdate?(appState)
                    }
                }
            }
        }

        // Stream ended — reconnect.
        DispatchQueue.main.async { [weak self] in
            self?.disconnect()
            self?.scheduleReconnect()
        }
    }

    private func scheduleReconnect() {
        guard !stopping else { return }
        reconnectTimer?.invalidate()
        reconnectTimer = Timer.scheduledTimer(withTimeInterval: backoff, repeats: false) { [weak self] _ in
            self?.connect()
        }
        backoff = min(backoff * 2, maxBackoff)
    }
}
