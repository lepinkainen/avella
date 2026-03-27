import Foundation

/// Full state snapshot from the Go daemon.
struct AppState: Codable {
    let status: String
    let processed: Int
    let dryRun: Bool
    let configPath: String
    let rules: [RuleInfo]
    let version: String
    let recentFiles: [RecentFile]

    enum CodingKeys: String, CodingKey {
        case status, processed, rules, version
        case dryRun = "dry_run"
        case configPath = "config_path"
        case recentFiles = "recent_files"
    }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        status = try c.decode(String.self, forKey: .status)
        processed = try c.decode(Int.self, forKey: .processed)
        dryRun = try c.decode(Bool.self, forKey: .dryRun)
        configPath = try c.decode(String.self, forKey: .configPath)
        rules = try c.decode([RuleInfo].self, forKey: .rules)
        version = try c.decode(String.self, forKey: .version)
        recentFiles = try c.decodeIfPresent([RecentFile].self, forKey: .recentFiles) ?? []
    }
}

/// A recently processed file.
struct RecentFile: Codable {
    let filename: String
    let rule: String
    let action: String
    let dryRun: Bool
    let time: String

    enum CodingKeys: String, CodingKey {
        case filename, rule, action, time
        case dryRun = "dry_run"
    }
}

/// A single rule for display.
struct RuleInfo: Codable {
    let name: String
    let actionType: String

    enum CodingKeys: String, CodingKey {
        case name
        case actionType = "action_type"
    }
}

/// The protocol version this client supports. Must match the daemon.
let supportedProtocolVersion = 1

/// Hello handshake data sent by the daemon on connect.
struct HelloData: Codable {
    let protocolVersion: Int

    enum CodingKeys: String, CodingKey {
        case protocolVersion = "protocol_version"
    }
}

/// Envelope for messages received from the server.
struct ServerMessage: Codable {
    let type: String
    let data: AppState?
}

/// Envelope with raw data for flexible decoding of different message types.
struct RawServerMessage: Codable {
    let type: String

    /// Parse a JSON line into a typed message, extracting the data field
    /// as raw bytes for type-specific decoding.
    static func parse(_ lineData: Data) -> (type: String, data: Data?)? {
        guard let obj = try? JSONSerialization.jsonObject(with: lineData) as? [String: Any],
              let type = obj["type"] as? String else {
            return nil
        }
        guard let dataVal = obj["data"] else {
            return (type: type, data: nil)
        }
        let dataBytes = try? JSONSerialization.data(withJSONObject: dataVal)
        return (type: type, data: dataBytes)
    }
}

/// Command sent from the tray to the daemon.
struct ClientCommand: Encodable {
    var type: String = "command"
    let command: String
}
