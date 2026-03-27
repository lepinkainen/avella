import Foundation
import XCTest
@testable import AvellaTrayLib

final class SocketClientTests: XCTestCase {

    func testInitialBackoff() {
        let client = SocketClient()
        XCTAssertEqual(client.backoff, 1.0)
    }

    func testBackoffDoublesOnReconnect() {
        let client = SocketClient()
        // Prevent actual timer from firing by marking as stopping after scheduling.
        client.stopping = false
        client.scheduleReconnect()
        XCTAssertEqual(client.backoff, 2.0)

        client.scheduleReconnect()
        XCTAssertEqual(client.backoff, 4.0)

        client.scheduleReconnect()
        XCTAssertEqual(client.backoff, 8.0)

        client.scheduleReconnect()
        XCTAssertEqual(client.backoff, 10.0, "Backoff should cap at maxBackoff")

        client.scheduleReconnect()
        XCTAssertEqual(client.backoff, 10.0, "Backoff should remain at maxBackoff")

        client.stop()
    }

    func testMaxBackoffValue() {
        let client = SocketClient()
        XCTAssertEqual(client.maxBackoff, 10.0)
    }

    func testStopPreventsReconnect() {
        let client = SocketClient()
        client.stop()
        XCTAssertTrue(client.stopping)

        let backoffBefore = client.backoff
        client.scheduleReconnect()
        // Backoff should not change because scheduleReconnect exits early when stopping.
        XCTAssertEqual(client.backoff, backoffBefore)
    }

    func testStartResetsStopping() {
        let client = SocketClient()
        client.stopping = true
        // start() sets stopping to false (and tries to connect, which will fail harmlessly).
        client.start()
        XCTAssertFalse(client.stopping)
        client.stop()
    }
}
