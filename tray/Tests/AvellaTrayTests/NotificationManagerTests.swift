import Foundation
import XCTest
@testable import AvellaTrayLib

final class NotificationManagerTests: XCTestCase {

    private func makeFile(
        _ name: String,
        rule: String = "test",
        action: String = "/dest",
        dryRun: Bool = false,
        time: String = "2026-03-27T10:00:00Z"
    ) -> RecentFile {
        let json = """
        {
            "filename": "\(name)",
            "rule": "\(rule)",
            "action": "\(action)",
            "dry_run": \(dryRun),
            "time": "\(time)"
        }
        """
        return try! JSONDecoder().decode(RecentFile.self, from: Data(json.utf8))
    }

    // MARK: - countNewFiles

    func testCountNewFilesEmptyPrevious() {
        let mgr = NotificationManager()
        let current = [makeFile("a.txt"), makeFile("b.txt")]
        XCTAssertEqual(mgr.countNewFiles(current: current, previous: []), 2)
    }

    func testCountNewFilesNoChange() {
        let mgr = NotificationManager()
        let files = [makeFile("a.txt", time: "T1"), makeFile("b.txt", time: "T2")]
        XCTAssertEqual(mgr.countNewFiles(current: files, previous: files), 0)
    }

    func testCountNewFilesOneNew() {
        let mgr = NotificationManager()
        let old = [makeFile("a.txt", time: "T1")]
        let current = [makeFile("b.txt", time: "T2"), makeFile("a.txt", time: "T1")]
        XCTAssertEqual(mgr.countNewFiles(current: current, previous: old), 1)
    }

    func testCountNewFilesMultipleNew() {
        let mgr = NotificationManager()
        let old = [makeFile("a.txt", time: "T1")]
        let current = [
            makeFile("c.txt", time: "T3"),
            makeFile("b.txt", time: "T2"),
            makeFile("a.txt", time: "T1"),
        ]
        XCTAssertEqual(mgr.countNewFiles(current: current, previous: old), 2)
    }

    func testCountNewFilesCompletelyDifferent() {
        let mgr = NotificationManager()
        let old = [makeFile("old.txt", time: "T0")]
        let current = [makeFile("x.txt", time: "T1"), makeFile("y.txt", time: "T2")]
        // No match found, so all are considered new.
        XCTAssertEqual(mgr.countNewFiles(current: current, previous: old), 2)
    }

    func testCountNewFilesMatchesOnAllThreeFields() {
        let mgr = NotificationManager()
        // Same filename and time but different rule — should NOT match.
        let old = [makeFile("a.txt", rule: "ruleA", time: "T1")]
        let current = [makeFile("a.txt", rule: "ruleB", time: "T1")]
        XCTAssertEqual(mgr.countNewFiles(current: current, previous: old), 1)
    }

    // MARK: - handleStateUpdate first-update skip

    func testFirstUpdateSkipsAndStoresFiles() {
        let mgr = NotificationManager()
        XCTAssertTrue(mgr.firstUpdate)

        let files = [makeFile("a.txt")]
        mgr.handleStateUpdate(recentFiles: files)

        XCTAssertFalse(mgr.firstUpdate)
        XCTAssertEqual(mgr.lastSeenFiles.count, 1)
        XCTAssertEqual(mgr.lastSeenFiles[0].filename, "a.txt")
    }

    func testSecondUpdateTracksFiles() {
        let mgr = NotificationManager()

        // First update — skipped.
        mgr.handleStateUpdate(recentFiles: [makeFile("a.txt", time: "T1")])

        // Second update — notifications disabled (setupDone is false), but state should still be tracked.
        let newFiles = [makeFile("b.txt", time: "T2"), makeFile("a.txt", time: "T1")]
        mgr.handleStateUpdate(recentFiles: newFiles)

        XCTAssertEqual(mgr.lastSeenFiles.count, 2)
        XCTAssertEqual(mgr.lastSeenFiles[0].filename, "b.txt")
    }
}
