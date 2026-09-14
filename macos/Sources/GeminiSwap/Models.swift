import Foundation

enum AccountType: String, Codable {
    case oauth = "oauth"
    case apiKey = "api_key"
}

struct OAuthData: Codable {
    var accessToken: String
    var refreshToken: String?
    var tokenType: String?
    var idToken: String?
    var expiryDate: Int64?
    var scope: String?
    var client: String?

    enum CodingKeys: String, CodingKey {
        case accessToken = "access_token"
        case refreshToken = "refresh_token"
        case tokenType = "token_type"
        case idToken = "id_token"
        case expiryDate = "expiry_date"
        case scope = "scope"
        case client = "client"
    }
}

struct QuotaBucket: Codable, Identifiable {
    var id: String {
        if let bid = bucketID, !bid.isEmpty {
            return bid
        }
        if let mid = modelID, !mid.isEmpty {
            return mid
        }
        return displayName
    }
    var bucketID: String?
    var modelID: String?
    var customDisplayName: String?
    var description: String?
    var window: String?
    var remainingAmount: Int?
    var remainingFraction: Double
    var limit: Int?
    var resetTime: String?

    enum CodingKeys: String, CodingKey {
        case bucketID = "bucket_id"
        case modelID = "model_id"
        case customDisplayName = "display_name"
        case description = "description"
        case window = "window"
        case remainingAmount = "remaining_amount"
        case remainingFraction = "remaining_fraction"
        case limit = "limit"
        case resetTime = "reset_time"
    }

    func resetDate() -> Date? {
        guard let resetTime, !resetTime.isEmpty else { return nil }
        return try? Date(resetTime, strategy: .iso8601)
    }

    func hasLocallyReset(at date: Date) -> Bool {
        guard remainingFraction < 1, let resetDate = resetDate() else { return false }
        return date >= resetDate
    }

    func effectiveRemainingFraction(at date: Date) -> Double {
        hasLocallyReset(at: date) ? 1 : remainingFraction
    }

    func percentage(at date: Date) -> Int {
        Int((effectiveRemainingFraction(at: date) * 100).rounded())
    }

    var percentage: Int {
        percentage(at: Date())
    }

    var displayName: String {
        if let name = customDisplayName, !name.isEmpty {
            return name
        }
        if let mid = modelID {
            if mid.contains("flash") {
                return "Gemini 2.5 Flash"
            } else if mid.contains("pro") {
                return "Gemini 2.5 Pro"
            }
            return mid
        }
        return "Limit"
    }
}

struct QuotaGroup: Codable, Identifiable {
    var id: String { displayName }
    var displayName: String
    var description: String?
    var buckets: [QuotaBucket]

    enum CodingKeys: String, CodingKey {
        case displayName = "display_name"
        case description = "description"
        case buckets = "buckets"
    }
}

struct QuotaInfo: Codable {
    var updatedAt: String
    var tier: String?
    var groups: [QuotaGroup]?
    var buckets: [QuotaBucket]?
    var description: String?

    enum CodingKeys: String, CodingKey {
        case updatedAt = "updated_at"
        case tier = "tier"
        case groups = "groups"
        case buckets = "buckets"
        case description = "description"
    }

    var allBuckets: [QuotaBucket] {
        if let g = groups, !g.isEmpty {
            return g.flatMap { $0.buckets }
        }
        return buckets ?? []
    }
}

struct Account: Codable, Identifiable {
    var id: String
    var type: AccountType
    var name: String
    var email: String?
    var picture: String?
    var projectID: String?
    var oauth: OAuthData?
    var apiKey: String?
    var rpmLimit: Int?
    var rpdLimit: Int?
    var lastQuota: QuotaInfo?
    var createdAt: String?
    var updatedAt: String?

    enum CodingKeys: String, CodingKey {
        case id = "id"
        case type = "type"
        case name = "name"
        case email = "email"
        case picture = "picture"
        case projectID = "project_id"
        case oauth = "oauth"
        case apiKey = "api_key"
        case rpmLimit = "rpm_limit"
        case rpdLimit = "rpd_limit"
        case lastQuota = "last_quota"
        case createdAt = "created_at"
        case updatedAt = "updated_at"
    }
}

struct StoreData: Codable {
    var version: Int
    var activeAccountID: String
    var accounts: [Account]
    var proxyPort: Int?
    var autoRotate: Bool?
    var lastUpdated: String?

    enum CodingKeys: String, CodingKey {
        case version = "version"
        case activeAccountID = "active_account_id"
        case accounts = "accounts"
        case proxyPort = "proxy_port"
        case autoRotate = "auto_rotate"
        case lastUpdated = "last_updated"
    }
}

struct RequestLog: Codable, Identifiable {
    var id: String { "\(timestamp)-\(accountID)-\(path)" }
    var timestamp: String
    var accountID: String
    var accountName: String
    var path: String
    var method: String
    var statusCode: Int
    var durationMs: Int64
    var rotated: Bool?

    enum CodingKeys: String, CodingKey {
        case timestamp = "timestamp"
        case accountID = "account_id"
        case accountName = "account_name"
        case path = "path"
        case method = "method"
        case statusCode = "status_code"
        case durationMs = "duration_ms"
        case rotated = "rotated"
    }
}

struct ProxyStats: Codable {
    var totalRequests: Int
    var rotations: Int
    var activeAccount: String
    var recentLogs: [RequestLog]
    var lastUpdated: String?

    enum CodingKeys: String, CodingKey {
        case totalRequests = "total_requests"
        case rotations = "rotations"
        case activeAccount = "active_account"
        case recentLogs = "recent_logs"
        case lastUpdated = "last_updated"
    }
}
