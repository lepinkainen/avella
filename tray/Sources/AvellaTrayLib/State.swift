import Foundation

extension JSONDecoder {
    /// A decoder configured for the Avella wire protocol: snake_case keys
    /// on the wire, camelCase properties in Swift.
    ///
    /// `.convertFromSnakeCase` transforms incoming JSON keys to camelCase
    /// BEFORE they are matched against `CodingKeys` raw values. This means
    /// any `CodingKeys` raw value used with this decoder must already be
    /// spelled in post-conversion camelCase (e.g. `"recentFiles"`), never
    /// in wire-format snake_case (e.g. `"recent_files"`) — a snake_case raw
    /// value will never match and will silently fail to decode that field.
    static func avella() -> JSONDecoder {
        let decoder = JSONDecoder()
        decoder.keyDecodingStrategy = .convertFromSnakeCase
        return decoder
    }
}

/// Full state snapshot from the Go daemon.
struct AppState: Codable, Equatable, Sendable {
    let status: String
    let processed: Int
    let dryRun: Bool
    let configPath: String
    let rules: [RuleInfo]
    let version: String
    private let recentFilesStorage: [RecentFile]?

    var recentFiles: [RecentFile] { recentFilesStorage ?? [] }

    // Raw values must be spelled in post-conversion camelCase, never wire
    // snake_case — see the doc comment on JSONDecoder.avella() above.
    private enum CodingKeys: String, CodingKey {
        case status, processed, dryRun, configPath, rules, version
        case recentFilesStorage = "recentFiles"
    }

    init(
        status: String,
        processed: Int,
        dryRun: Bool,
        configPath: String,
        rules: [RuleInfo],
        version: String,
        recentFiles: [RecentFile] = []
    ) {
        self.status = status
        self.processed = processed
        self.dryRun = dryRun
        self.configPath = configPath
        self.rules = rules
        self.version = version
        self.recentFilesStorage = recentFiles
    }
}

/// A recently processed file.
///
/// Not `Identifiable`: the daemon stamps `time` with RFC3339 (1-second
/// granularity), so two same-named files matched by the same rule in the
/// same second are indistinguishable by any derived string id — even
/// including `action`/`dryRun` cannot make truly identical entries unique.
/// Views must use positional identity (`ForEach(Array(...enumerated()), id: \.offset)`).
struct RecentFile: Codable, Equatable, Hashable, Sendable {
    let filename: String
    let rule: String
    let action: String
    let dryRun: Bool
    let time: String
}

/// A single rule for display.
struct RuleInfo: Codable, Identifiable, Equatable, Hashable, Sendable {
    let name: String
    let actionType: String

    var id: String { name }
}

/// The protocol version this client supports. Must match the daemon.
let supportedProtocolVersion = 1

/// Hello handshake data sent by the daemon on connect.
struct HelloData: Codable, Sendable {
    let protocolVersion: Int
}

/// A message received from the server, decoded from its `type` envelope.
enum ServerMessage: Decodable, Sendable {
    case hello(HelloData)
    case state(AppState)
    case unknown(type: String)

    private enum CodingKeys: String, CodingKey {
        case type, data
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        let type = try container.decode(String.self, forKey: .type)
        switch type {
        case "hello":
            let data = try container.decode(HelloData.self, forKey: .data)
            self = .hello(data)
        case "state":
            let data = try container.decode(AppState.self, forKey: .data)
            self = .state(data)
        default:
            self = .unknown(type: type)
        }
    }
}

/// Commands the tray can send to the daemon.
enum DaemonCommand: String, Sendable {
    case toggleDryRun = "toggle_dry_run"
    case openConfig = "open_config"
    case quit
}

/// Command sent from the tray to the daemon.
struct ClientCommand: Encodable {
    var type: String = "command"
    let command: String

    init(_ command: DaemonCommand) {
        self.command = command.rawValue
    }
}
