import XCTest
@testable import GeminiSwap

final class QuotaBucketTests: XCTestCase {
    private let now = try! Date("2026-09-14T12:00:00Z", strategy: .iso8601)

    func testPastResetRestoresQuotaLocally() {
        let bucket = makeBucket(fraction: 0, resetTime: "2026-09-14T11:00:00Z")

        XCTAssertTrue(bucket.hasLocallyReset(at: now))
        XCTAssertEqual(bucket.effectiveRemainingFraction(at: now), 1)
        XCTAssertEqual(bucket.percentage(at: now), 100)
    }

    func testFutureResetKeepsLastKnownQuota() {
        let bucket = makeBucket(fraction: 0.23, resetTime: "2026-09-14T13:00:00Z")

        XCTAssertFalse(bucket.hasLocallyReset(at: now))
        XCTAssertEqual(bucket.effectiveRemainingFraction(at: now), 0.23)
        XCTAssertEqual(bucket.percentage(at: now), 23)
    }

    func testInvalidResetTimeKeepsLastKnownQuota() {
        let bucket = makeBucket(fraction: 0.4, resetTime: "not-a-date")

        XCTAssertFalse(bucket.hasLocallyReset(at: now))
        XCTAssertEqual(bucket.effectiveRemainingFraction(at: now), 0.4)
    }

    private func makeBucket(fraction: Double, resetTime: String) -> QuotaBucket {
        QuotaBucket(
            bucketID: "test",
            modelID: nil,
            customDisplayName: "Five Hour Limit Remaining",
            description: nil,
            window: nil,
            remainingAmount: nil,
            remainingFraction: fraction,
            limit: nil,
            resetTime: resetTime
        )
    }
}
