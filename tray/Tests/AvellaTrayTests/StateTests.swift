import Foundation
import XCTest
@testable import AvellaTrayLib

final class StateTests: XCTestCase {

    // MARK: - AppState decoding

    func testDecodeFullState() throws {
        let json = """
        {
            "status": "watching",
            "processed": 42,
            "dry_run": true,
            "config_path": "/home/user/.config/avella/config.yaml",
            "rules": [
                {"name": "videos", "action_type": "move"},
                {"name": "music", "action_type": "scp"}
            ],
            "version": "1.2.3",
            "recent_files": [
                {
                    "filename": "video.mp4",
                    "rule": "videos",
                    "action": "/media/videos/video.mp4",
                    "dry_run": false,
                    "time": "2026-03-27T10:00:00Z"
                }
            ]
        }
        """
        let state = try JSONDecoder().decode(AppState.self, from: Data(json.utf8))

        XCTAssertEqual(state.status, "watching")
        XCTAssertEqual(state.processed, 42)
        XCTAssertTrue(state.dryRun)
        XCTAssertEqual(state.configPath, "/home/user/.config/avella/config.yaml")
        XCTAssertEqual(state.rules.count, 2)
        XCTAssertEqual(state.rules[0].name, "videos")
        XCTAssertEqual(state.rules[0].actionType, "move")
        XCTAssertEqual(state.rules[1].name, "music")
        XCTAssertEqual(state.rules[1].actionType, "scp")
        XCTAssertEqual(state.version, "1.2.3")
        XCTAssertEqual(state.recentFiles.count, 1)
        XCTAssertEqual(state.recentFiles[0].filename, "video.mp4")
    }

    func testDecodeStateMissingRecentFiles() throws {
        let json = """
        {
            "status": "idle",
            "processed": 0,
            "dry_run": false,
            "config_path": "/tmp/config.yaml",
            "rules": [],
            "version": "dev"
        }
        """
        let state = try JSONDecoder().decode(AppState.self, from: Data(json.utf8))

        XCTAssertEqual(state.status, "idle")
        XCTAssertEqual(state.processed, 0)
        XCTAssertFalse(state.dryRun)
        XCTAssertEqual(state.recentFiles, [])
    }

    func testDecodeStateEmptyRecentFiles() throws {
        let json = """
        {
            "status": "watching",
            "processed": 5,
            "dry_run": false,
            "config_path": "/tmp/config.yaml",
            "rules": [],
            "version": "1.0.0",
            "recent_files": []
        }
        """
        let state = try JSONDecoder().decode(AppState.self, from: Data(json.utf8))
        XCTAssertEqual(state.recentFiles, [])
    }

    func testDecodeStateInvalidJSONThrows() {
        let json = "{ not valid json }"
        XCTAssertThrowsError(try JSONDecoder().decode(AppState.self, from: Data(json.utf8)))
    }

    func testDecodeStateMissingRequiredFieldThrows() {
        let json = """
        {
            "status": "watching",
            "processed": 0,
            "dry_run": false,
            "rules": [],
            "version": "1.0.0"
        }
        """
        // config_path is missing
        XCTAssertThrowsError(try JSONDecoder().decode(AppState.self, from: Data(json.utf8)))
    }

    // MARK: - RecentFile decoding

    func testDecodeRecentFile() throws {
        let json = """
        {
            "filename": "report.pdf",
            "rule": "documents",
            "action": "/archive/report.pdf",
            "dry_run": true,
            "time": "2026-03-27T12:30:00Z"
        }
        """
        let file = try JSONDecoder().decode(RecentFile.self, from: Data(json.utf8))

        XCTAssertEqual(file.filename, "report.pdf")
        XCTAssertEqual(file.rule, "documents")
        XCTAssertEqual(file.action, "/archive/report.pdf")
        XCTAssertTrue(file.dryRun)
        XCTAssertEqual(file.time, "2026-03-27T12:30:00Z")
    }

    // MARK: - RuleInfo decoding

    func testDecodeRuleInfo() throws {
        let json = """
        {"name": "downloads", "action_type": "exec"}
        """
        let rule = try JSONDecoder().decode(RuleInfo.self, from: Data(json.utf8))

        XCTAssertEqual(rule.name, "downloads")
        XCTAssertEqual(rule.actionType, "exec")
    }

    // MARK: - ServerMessage decoding

    func testDecodeServerMessageWithState() throws {
        let json = """
        {
            "type": "state",
            "data": {
                "status": "watching",
                "processed": 1,
                "dry_run": false,
                "config_path": "/tmp/c.yaml",
                "rules": [],
                "version": "1.0.0"
            }
        }
        """
        let msg = try JSONDecoder().decode(ServerMessage.self, from: Data(json.utf8))

        XCTAssertEqual(msg.type, "state")
        XCTAssertNotNil(msg.data)
        XCTAssertEqual(msg.data?.status, "watching")
    }

    func testDecodeServerMessageWithoutData() throws {
        let json = """
        {"type": "ping"}
        """
        let msg = try JSONDecoder().decode(ServerMessage.self, from: Data(json.utf8))

        XCTAssertEqual(msg.type, "ping")
        XCTAssertNil(msg.data)
    }

    // MARK: - HelloData decoding

    func testDecodeHelloData() throws {
        let json = """
        {"protocol_version": 1}
        """
        let hello = try JSONDecoder().decode(HelloData.self, from: Data(json.utf8))
        XCTAssertEqual(hello.protocolVersion, 1)
    }

    // MARK: - RawServerMessage parsing

    func testParseHelloMessage() {
        let json = """
        {"type": "hello", "data": {"protocol_version": 1}}
        """
        let result = RawServerMessage.parse(Data(json.utf8))
        XCTAssertNotNil(result)
        XCTAssertEqual(result?.type, "hello")
        XCTAssertNotNil(result?.data)

        let hello = try? JSONDecoder().decode(HelloData.self, from: result!.data!)
        XCTAssertEqual(hello?.protocolVersion, 1)
    }

    func testParseStateMessage() {
        let json = """
        {"type": "state", "data": {"status": "Idle", "processed": 0, "dry_run": false, "config_path": "/tmp/c.yaml", "rules": [], "version": "1.0.0"}}
        """
        let result = RawServerMessage.parse(Data(json.utf8))
        XCTAssertNotNil(result)
        XCTAssertEqual(result?.type, "state")

        let state = try? JSONDecoder().decode(AppState.self, from: result!.data!)
        XCTAssertEqual(state?.status, "Idle")
    }

    func testParseMessageWithoutData() {
        let json = """
        {"type": "ping"}
        """
        let result = RawServerMessage.parse(Data(json.utf8))
        XCTAssertNotNil(result)
        XCTAssertEqual(result?.type, "ping")
        XCTAssertNil(result?.data)
    }

    func testParseInvalidJSON() {
        let result = RawServerMessage.parse(Data("not json".utf8))
        XCTAssertNil(result)
    }

    func testSupportedProtocolVersion() {
        XCTAssertEqual(supportedProtocolVersion, 1)
    }

    // MARK: - ClientCommand encoding

    func testEncodeClientCommand() throws {
        let cmd = ClientCommand(command: "toggle_dry_run")
        let data = try JSONEncoder().encode(cmd)
        let dict = try JSONDecoder().decode([String: String].self, from: data)

        XCTAssertEqual(dict["type"], "command")
        XCTAssertEqual(dict["command"], "toggle_dry_run")
    }
}

// Equatable conformance for test assertions.
extension RecentFile: Equatable {
    public static func == (lhs: RecentFile, rhs: RecentFile) -> Bool {
        lhs.filename == rhs.filename
            && lhs.rule == rhs.rule
            && lhs.action == rhs.action
            && lhs.dryRun == rhs.dryRun
            && lhs.time == rhs.time
    }
}
