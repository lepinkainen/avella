import Foundation
import XCTest
@testable import AvellaTrayLib

@MainActor
final class TrayViewModelTests: XCTestCase {

    private func makeState(
        status: String = "watching",
        processed: Int = 5,
        dryRun: Bool = false,
        configPath: String = "/tmp/config.yaml",
        rules: [[String: String]] = [["name": "videos", "action_type": "move"]],
        version: String = "1.0.0",
        recentFiles: [[String: Any]] = []
    ) -> AppState {
        var rulesJSON = "["
        rulesJSON += rules.map { r in
            "{\"name\": \"\(r["name"]!)\", \"action_type\": \"\(r["action_type"]!)\"}"
        }.joined(separator: ",")
        rulesJSON += "]"

        var recentJSON = "["
        recentJSON += recentFiles.map { f in
            """
            {"filename":"\(f["filename"]!)","rule":"\(f["rule"]!)","action":"\(f["action"]!)","dry_run":\(f["dry_run"]!),"time":"\(f["time"]!)"}
            """
        }.joined(separator: ",")
        recentJSON += "]"

        let json = """
        {
            "status": "\(status)",
            "processed": \(processed),
            "dry_run": \(dryRun),
            "config_path": "\(configPath)",
            "rules": \(rulesJSON),
            "version": "\(version)",
            "recent_files": \(recentJSON)
        }
        """
        return try! JSONDecoder.avella().decode(AppState.self, from: Data(json.utf8))
    }

    // MARK: - Initial state

    func testInitialStateIsDisconnected() {
        let vm = TrayViewModel()
        XCTAssertEqual(vm.status, "Disconnected")
        XCTAssertEqual(vm.processed, 0)
        XCTAssertFalse(vm.dryRun)
        XCTAssertTrue(vm.recentFiles.isEmpty)
        XCTAssertTrue(vm.rules.isEmpty)
        XCTAssertFalse(vm.isConnected)
    }

    // MARK: - update(state:)

    func testUpdateReflectsState() {
        let vm = TrayViewModel()
        let state = makeState(status: "watching", processed: 42, dryRun: true, version: "1.2.3")
        vm.update(state: state)

        XCTAssertEqual(vm.status, "watching")
        XCTAssertEqual(vm.processed, 42)
        XCTAssertTrue(vm.dryRun)
        XCTAssertEqual(vm.version, "1.2.3")
        XCTAssertTrue(vm.isConnected)
    }

    func testUpdateWithDryRunOff() {
        let vm = TrayViewModel()
        vm.update(state: makeState(dryRun: false))
        XCTAssertFalse(vm.dryRun)
    }

    func testUpdateWithDryRunOn() {
        let vm = TrayViewModel()
        vm.update(state: makeState(dryRun: true))
        XCTAssertTrue(vm.dryRun)
    }

    func testRecentFilesPopulated() {
        let vm = TrayViewModel()
        let state = makeState(recentFiles: [
            ["filename": "clip.mp4", "rule": "videos", "action": "/media/clip.mp4", "dry_run": false, "time": "T1"],
            ["filename": "song.mp3", "rule": "music", "action": "/media/song.mp3", "dry_run": true, "time": "T2"],
        ])
        vm.update(state: state)

        XCTAssertEqual(vm.recentFiles.count, 2)
        XCTAssertEqual(vm.recentFiles[0].filename, "clip.mp4")
        XCTAssertEqual(vm.recentFiles[1].filename, "song.mp3")
        XCTAssertTrue(vm.recentFiles[1].dryRun)
    }

    func testRulesPopulated() {
        let vm = TrayViewModel()
        let state = makeState(rules: [
            ["name": "videos", "action_type": "move"],
            ["name": "music", "action_type": "scp"],
        ])
        vm.update(state: state)

        XCTAssertEqual(vm.rules.count, 2)
        XCTAssertEqual(vm.rules[0].name, "videos")
        XCTAssertEqual(vm.rules[0].actionType, "move")
        XCTAssertEqual(vm.rules[1].name, "music")
        XCTAssertEqual(vm.rules[1].actionType, "scp")
    }

    // MARK: - setDisconnected

    func testSetDisconnectedResetsState() {
        let vm = TrayViewModel()
        vm.update(state: makeState(status: "watching", processed: 10, dryRun: true))
        XCTAssertTrue(vm.isConnected)

        vm.setDisconnected()

        XCTAssertEqual(vm.status, "Disconnected")
        XCTAssertEqual(vm.processed, 0)
        XCTAssertFalse(vm.dryRun)
        XCTAssertTrue(vm.recentFiles.isEmpty)
        XCTAssertTrue(vm.rules.isEmpty)
        XCTAssertFalse(vm.isConnected)
    }

    // MARK: - setProtocolMismatch

    func testSetProtocolMismatch() {
        let vm = TrayViewModel()
        vm.setProtocolMismatch(daemon: 2, tray: 1)

        XCTAssertTrue(vm.status.contains("Protocol mismatch"))
        XCTAssertTrue(vm.status.contains("v2"))
        XCTAssertTrue(vm.status.contains("v1"))
        XCTAssertFalse(vm.isConnected)
    }

    // MARK: - Actions

    func testActionsAreFired() {
        let vm = TrayViewModel()

        var firedActions: [TrayAction] = []
        vm.onAction = { action in firedActions.append(action) }

        vm.perform(.toggleDryRun)
        vm.perform(.toggleNotifications)
        vm.perform(.openConfig)
        vm.perform(.quit)

        XCTAssertEqual(firedActions.count, 4)
        guard case .toggleDryRun = firedActions[0] else { return XCTFail("expected toggleDryRun") }
        guard case .toggleNotifications = firedActions[1] else { return XCTFail("expected toggleNotifications") }
        guard case .openConfig = firedActions[2] else { return XCTFail("expected openConfig") }
        guard case .quit = firedActions[3] else { return XCTFail("expected quit") }
    }

    // MARK: - Notifications toggle

    func testNotificationsEnabledReflectsManagerWithoutManualSync() {
        let mgr = NotificationManager.shared
        let original = mgr.isEnabled
        defer { mgr.setEnabled(original) }

        let vm = TrayViewModel()

        mgr.setEnabled(true)
        XCTAssertTrue(vm.notificationsEnabled)

        mgr.setEnabled(false)
        XCTAssertFalse(vm.notificationsEnabled)
    }

    // MARK: - ConnectionState

    func testIsConnectedFalseForMismatch() {
        let vm = TrayViewModel()
        vm.setProtocolMismatch(daemon: 2, tray: 1)
        XCTAssertFalse(vm.isConnected)
    }

    func testIsConnectedTrueAfterUpdate() {
        let vm = TrayViewModel()
        vm.update(state: makeState())
        XCTAssertTrue(vm.isConnected)
    }
}
