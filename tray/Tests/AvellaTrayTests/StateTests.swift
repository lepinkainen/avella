import Foundation
import XCTest
@testable import AvellaTrayLib

final class StateTests: XCTestCase {

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
    let state = try JSONDecoder.avella().decode(AppState.self, from: Data(json.utf8))

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
    let state = try JSONDecoder.avella().decode(AppState.self, from: Data(json.utf8))

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
    let state = try JSONDecoder.avella().decode(AppState.self, from: Data(json.utf8))
    XCTAssertEqual(state.recentFiles, [])
  }

  func testDecodeStateInvalidJSONThrows() {
    let json = "{ not valid json }"
    XCTAssertThrowsError(try JSONDecoder.avella().decode(AppState.self, from: Data(json.utf8)))
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
    XCTAssertThrowsError(try JSONDecoder.avella().decode(AppState.self, from: Data(json.utf8)))
  }

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
    let file = try JSONDecoder.avella().decode(RecentFile.self, from: Data(json.utf8))

    XCTAssertEqual(file.filename, "report.pdf")
    XCTAssertEqual(file.rule, "documents")
    XCTAssertEqual(file.action, "/archive/report.pdf")
    XCTAssertTrue(file.dryRun)
    XCTAssertEqual(file.time, "2026-03-27T12:30:00Z")
  }

  func testRecentFileNoLongerIdentifiable() throws {
    // RecentFile intentionally has no derived id: filename+time+rule can
    // collide (daemon timestamps are 1s granularity), so views must use
    // positional identity instead. This test just documents equality
    // still works for otherwise-identical entries.
    let a = try JSONDecoder.avella().decode(RecentFile.self, from: Data("""
      {"filename": "a.txt", "rule": "r", "action": "act", "dry_run": false, "time": "T1"}
      """.utf8))
    let b = try JSONDecoder.avella().decode(RecentFile.self, from: Data("""
      {"filename": "a.txt", "rule": "r", "action": "act", "dry_run": false, "time": "T1"}
      """.utf8))
    XCTAssertEqual(a, b)
  }

  func testDecodeRuleInfo() throws {
    let json = """
      {"name": "downloads", "action_type": "exec"}
      """
    let rule = try JSONDecoder.avella().decode(RuleInfo.self, from: Data(json.utf8))

    XCTAssertEqual(rule.name, "downloads")
    XCTAssertEqual(rule.actionType, "exec")
    XCTAssertEqual(rule.id, "downloads")
  }

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
    let msg = try JSONDecoder.avella().decode(ServerMessage.self, from: Data(json.utf8))

    guard case .state(let state) = msg else {
      return XCTFail("expected .state")
    }
    XCTAssertEqual(state.status, "watching")
  }

  func testDecodeServerMessageUnknownType() throws {
    let json = """
      {"type": "ping"}
      """
    let msg = try JSONDecoder.avella().decode(ServerMessage.self, from: Data(json.utf8))

    guard case .unknown(let type) = msg else {
      return XCTFail("expected .unknown")
    }
    XCTAssertEqual(type, "ping")
  }

  func testDecodeServerMessageHello() throws {
    let json = """
      {"type": "hello", "data": {"protocol_version": 1}}
      """
    let msg = try JSONDecoder.avella().decode(ServerMessage.self, from: Data(json.utf8))

    guard case .hello(let hello) = msg else {
      return XCTFail("expected .hello")
    }
    XCTAssertEqual(hello.protocolVersion, 1)
  }

  func testDecodeServerMessageHelloMissingDataThrows() {
    let json = """
      {"type": "hello"}
      """
    XCTAssertThrowsError(try JSONDecoder.avella().decode(ServerMessage.self, from: Data(json.utf8)))
  }

  func testDecodeServerMessageStateMalformedDataThrows() {
    let json = """
      {"type": "state", "data": {"status": "watching"}}
      """
    XCTAssertThrowsError(try JSONDecoder.avella().decode(ServerMessage.self, from: Data(json.utf8)))
  }

  func testDecodeHelloData() throws {
    let json = """
      {"protocol_version": 1}
      """
    let hello = try JSONDecoder.avella().decode(HelloData.self, from: Data(json.utf8))
    XCTAssertEqual(hello.protocolVersion, 1)
  }

  func testSupportedProtocolVersion() {
    XCTAssertEqual(supportedProtocolVersion, 1)
  }

  func testEncodeClientCommand() throws {
    let cmd = ClientCommand(.toggleDryRun)
    let data = try JSONEncoder().encode(cmd)
    let dict = try JSONDecoder().decode([String: String].self, from: data)

    XCTAssertEqual(dict["type"], "command")
    XCTAssertEqual(dict["command"], "toggle_dry_run")
  }

  func testEncodeAllClientCommands() throws {
    let expectations: [(DaemonCommand, String)] = [
      (.toggleDryRun, "toggle_dry_run"),
      (.openConfig, "open_config"),
      (.quit, "quit"),
    ]
    for (command, expected) in expectations {
      let cmd = ClientCommand(command)
      let data = try JSONEncoder().encode(cmd)
      let dict = try JSONDecoder().decode([String: String].self, from: data)
      XCTAssertEqual(dict["command"], expected)
      XCTAssertEqual(dict["type"], "command")
    }
  }

  func testLineBufferSingleCompleteLine() {
    var buffer = LineBuffer()
    let lines = buffer.append(Data("hello\n".utf8))
    XCTAssertEqual(lines.map { String(decoding: $0, as: UTF8.self) }, ["hello"])
  }

  func testLineBufferPartialChunkAcrossTwoAppends() {
    var buffer = LineBuffer()
    let first = buffer.append(Data("hel".utf8))
    XCTAssertEqual(first, [])

    let second = buffer.append(Data("lo\n".utf8))
    XCTAssertEqual(second.map { String(decoding: $0, as: UTF8.self) }, ["hello"])
  }

  func testLineBufferMultipleCompleteLinesInOneChunk() {
    var buffer = LineBuffer()
    let lines = buffer.append(Data("one\ntwo\nthree\n".utf8))
    XCTAssertEqual(lines.map { String(decoding: $0, as: UTF8.self) }, ["one", "two", "three"])
  }

  func testLineBufferEmptyLines() {
    var buffer = LineBuffer()
    let lines = buffer.append(Data("a\n\n\nb\n".utf8))
    XCTAssertEqual(lines.map { String(decoding: $0, as: UTF8.self) }, ["a", "", "", "b"])
  }

  func testLineBufferLeftoverPartialDataRetainedWithNoTrailingNewline() {
    var buffer = LineBuffer()
    let lines = buffer.append(Data("complete\nincomplete".utf8))
    XCTAssertEqual(lines.map { String(decoding: $0, as: UTF8.self) }, ["complete"])

    // Nothing more arrives with a newline yet — leftover stays buffered.
    let more = buffer.append(Data())
    XCTAssertEqual(more, [])

    let final = buffer.append(Data(" data\n".utf8))
    XCTAssertEqual(final.map { String(decoding: $0, as: UTF8.self) }, ["incomplete data"])
  }

  func testBackoffDoublingSequence() {
    var backoff = Backoff()
    XCTAssertEqual(backoff.next(), 1)
    XCTAssertEqual(backoff.next(), 2)
    XCTAssertEqual(backoff.next(), 4)
    XCTAssertEqual(backoff.next(), 8)
    XCTAssertEqual(backoff.next(), 10, "should cap at 10")
    XCTAssertEqual(backoff.next(), 10, "should remain capped at 10")
  }

  func testBackoffReset() {
    var backoff = Backoff()
    _ = backoff.next()
    _ = backoff.next()
    _ = backoff.next()
    backoff.reset()
    XCTAssertEqual(backoff.next(), 1)
  }

}
