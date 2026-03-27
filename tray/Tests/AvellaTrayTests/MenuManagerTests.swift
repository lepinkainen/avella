import AppKit
import Foundation
import XCTest
@testable import AvellaTrayLib

final class MenuManagerTests: XCTestCase {

    private func makeStatusItem() -> NSStatusItem {
        NSStatusBar.system.statusItem(withLength: NSStatusItem.squareLength)
    }

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
        return try! JSONDecoder().decode(AppState.self, from: Data(json.utf8))
    }

    func testInitialStateIsDisconnected() {
        let item = makeStatusItem()
        _ = MenuManager(statusItem: item)

        let menu = item.menu!
        let statusItem = menu.items[0]
        XCTAssertEqual(statusItem.title, "Disconnected")
        XCTAssertFalse(statusItem.isEnabled)

        NSStatusBar.system.removeStatusItem(item)
    }

    func testUpdateReflectsState() {
        let item = makeStatusItem()
        let mgr = MenuManager(statusItem: item)
        let state = makeState(status: "watching", processed: 42, dryRun: true)

        mgr.update(state: state)

        let menu = item.menu!
        XCTAssertEqual(menu.items[0].title, "watching")
        XCTAssertEqual(menu.items[1].title, "Processed: 42 files")

        // Dry-run menu item (after separator at index 3).
        let dryRunItem = menu.items[4]
        XCTAssertEqual(dryRunItem.state, .on)
        XCTAssertTrue(dryRunItem.isEnabled)

        NSStatusBar.system.removeStatusItem(item)
    }

    func testUpdateWithDryRunOff() {
        let item = makeStatusItem()
        let mgr = MenuManager(statusItem: item)
        let state = makeState(dryRun: false)

        mgr.update(state: state)

        let dryRunItem = item.menu!.items[4]
        XCTAssertEqual(dryRunItem.state, .off)

        NSStatusBar.system.removeStatusItem(item)
    }

    func testSetDisconnectedResetsState() {
        let item = makeStatusItem()
        let mgr = MenuManager(statusItem: item)

        // First connect.
        mgr.update(state: makeState(status: "watching", processed: 10))

        // Then disconnect.
        mgr.setDisconnected()

        let menu = item.menu!
        XCTAssertEqual(menu.items[0].title, "Disconnected")
        XCTAssertEqual(menu.items[1].title, "Processed: 0 files")
        XCTAssertFalse(menu.items[0].isEnabled)

        NSStatusBar.system.removeStatusItem(item)
    }

    func testRulesSubmenuPopulated() {
        let item = makeStatusItem()
        let mgr = MenuManager(statusItem: item)
        let state = makeState(rules: [
            ["name": "videos", "action_type": "move"],
            ["name": "music", "action_type": "scp"],
        ])

        mgr.update(state: state)

        // Rules menu item is at index 7 (after second separator).
        let rulesItem = item.menu!.items[7]
        XCTAssertEqual(rulesItem.title, "Rules")
        XCTAssertTrue(rulesItem.isEnabled)
        XCTAssertEqual(rulesItem.submenu?.items.count, 2)
        XCTAssertEqual(rulesItem.submenu?.items[0].title, "videos: move")
        XCTAssertEqual(rulesItem.submenu?.items[1].title, "music: scp")

        NSStatusBar.system.removeStatusItem(item)
    }

    func testRecentFilesSubmenu() {
        let item = makeStatusItem()
        let mgr = MenuManager(statusItem: item)
        let state = makeState(recentFiles: [
            ["filename": "clip.mp4", "rule": "videos", "action": "/media/clip.mp4", "dry_run": false, "time": "T1"],
        ])

        mgr.update(state: state)

        let recentItem = item.menu!.items[2]
        XCTAssertEqual(recentItem.title, "Recent")
        XCTAssertTrue(recentItem.isEnabled)
        XCTAssertEqual(recentItem.submenu?.items.count, 1)
        XCTAssertTrue(recentItem.submenu!.items[0].title.contains("clip.mp4"))

        NSStatusBar.system.removeStatusItem(item)
    }

    func testRecentFilesEmptyShowsNone() {
        let item = makeStatusItem()
        let mgr = MenuManager(statusItem: item)
        let state = makeState(recentFiles: [])

        mgr.update(state: state)

        let recentItem = item.menu!.items[2]
        XCTAssertEqual(recentItem.submenu?.items.count, 1)
        XCTAssertEqual(recentItem.submenu?.items[0].title, "(none)")

        NSStatusBar.system.removeStatusItem(item)
    }

    func testCallbacksAreFired() {
        let item = makeStatusItem()
        let mgr = MenuManager(statusItem: item)

        var dryRunCalled = false
        var notifCalled = false
        var configCalled = false
        mgr.onToggleDryRun = { dryRunCalled = true }
        mgr.onToggleNotifications = { notifCalled = true }
        mgr.onOpenConfig = { configCalled = true }
        // Update state to wire up the menu item targets/actions.
        mgr.update(state: makeState())

        // Simulate menu clicks by invoking the action selectors.
        let menu = item.menu!
        // Dry-run is at index 4, notifications at 5, open config at 8, quit at 10.
        if let target = menu.items[4].target, let action = menu.items[4].action {
            _ = target.perform(action)
        }
        if let target = menu.items[5].target, let action = menu.items[5].action {
            _ = target.perform(action)
        }
        if let target = menu.items[8].target, let action = menu.items[8].action {
            _ = target.perform(action)
        }

        XCTAssertTrue(dryRunCalled)
        XCTAssertTrue(notifCalled)
        XCTAssertTrue(configCalled)

        NSStatusBar.system.removeStatusItem(item)
    }

    func testProtocolMismatchShowsError() {
        let item = makeStatusItem()
        let mgr = MenuManager(statusItem: item)

        mgr.setProtocolMismatch(daemon: 2, tray: 1)

        let menu = item.menu!
        XCTAssertTrue(menu.items[0].title.contains("Protocol mismatch"))
        XCTAssertTrue(menu.items[0].title.contains("v2"))
        XCTAssertTrue(menu.items[0].title.contains("v1"))
        XCTAssertFalse(menu.items[0].isEnabled)

        // Quit should still work.
        let quitItem = menu.items[10]
        XCTAssertEqual(quitItem.title, "Quit")
        XCTAssertTrue(quitItem.isEnabled)

        NSStatusBar.system.removeStatusItem(item)
    }
}
