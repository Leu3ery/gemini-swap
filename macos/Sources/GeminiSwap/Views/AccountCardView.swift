import SwiftUI

struct AccountCardView: View {
    let account: Account
    let isActive: Bool
    let onSwitch: () -> Void
    let onRemove: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            // Header
            HStack(alignment: .center) {
                // Type Icon / Avatar
                ZStack {
                    Circle()
                        .fill(isActive ? Color.blue.opacity(0.2) : Color.gray.opacity(0.15))
                        .frame(width: 42, height: 42)

                    Image(systemName: account.type == .oauth ? "person.crop.circle.fill" : "key.fill")
                        .font(.system(size: 20))
                        .foregroundColor(isActive ? .blue : .secondary)
                }

                VStack(alignment: .leading, spacing: 3) {
                    HStack(spacing: 8) {
                        Text(account.name)
                            .font(.system(size: 15, weight: .semibold))
                            .lineLimit(1)

                        if isActive {
                            HStack(spacing: 4) {
                                Circle()
                                    .fill(Color.green)
                                    .frame(width: 7, height: 7)
                                Text("ACTIVE")
                                    .font(.system(size: 10, weight: .bold))
                                    .foregroundColor(.green)
                            }
                            .padding(.horizontal, 8)
                            .padding(.vertical, 3)
                            .background(Color.green.opacity(0.15))
                            .cornerRadius(6)
                        }

                        Text(account.type == .oauth ? "Google OAuth" : "API Key")
                            .font(.system(size: 10, weight: .medium))
                            .padding(.horizontal, 6)
                            .padding(.vertical, 2)
                            .background(Color.secondary.opacity(0.15))
                            .cornerRadius(4)
                    }

                    if let email = account.email, !email.isEmpty {
                        Text(email)
                            .font(.system(size: 12))
                            .foregroundColor(.secondary)
                    }
                }

                Spacer()

                // Actions
                HStack(spacing: 10) {
                    if !isActive {
                        Button(action: onSwitch) {
                            HStack(spacing: 4) {
                                Image(systemName: "arrow.triangle.swap")
                                Text("Switch")
                            }
                        }
                        .buttonStyle(.borderedProminent)
                        .tint(.blue)
                        .controlSize(.small)
                    }

                    Button(action: onRemove) {
                        Image(systemName: "trash")
                            .foregroundColor(.red.opacity(0.8))
                    }
                    .buttonStyle(.plain)
                    .help("Remove account")
                }
            }

            // Quotas
            if let quota = account.lastQuota, !quota.buckets.isEmpty {
                Divider()
                    .padding(.vertical, 2)

                VStack(spacing: 10) {
                    ForEach(quota.buckets) { bucket in
                        QuotaBucketBar(bucket: bucket)
                    }
                }
            } else {
                Text("No quota information retrieved yet. Click Refresh to query Google.")
                    .font(.system(size: 12))
                    .foregroundColor(.secondary)
                    .italic()
                    .padding(.top, 4)
            }
        }
        .padding(16)
        .background(
            RoundedRectangle(cornerRadius: 12)
                .fill(Color(NSColor.controlBackgroundColor))
        )
        .overlay(
            RoundedRectangle(cornerRadius: 12)
                .stroke(isActive ? Color.blue.opacity(0.6) : Color.gray.opacity(0.2), lineWidth: isActive ? 2 : 1)
        )
        .shadow(color: isActive ? Color.blue.opacity(0.15) : Color.clear, radius: 8, x: 0, y: 2)
    }
}

struct QuotaBucketBar: View {
    let bucket: QuotaBucket

    var barColor: Color {
        if bucket.percentage < 25 {
            return .red
        } else if bucket.percentage < 60 {
            return .orange
        }
        return .green
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack {
                Text(bucket.displayName)
                    .font(.system(size: 12, weight: .medium))
                Spacer()
                Text("\(bucket.percentage)% (\(bucket.remainingAmount) / \(bucket.limit))")
                    .font(.system(size: 12, weight: .semibold))
                    .foregroundColor(barColor)
            }

            GeometryReader { geo in
                ZStack(alignment: .leading) {
                    RoundedRectangle(cornerRadius: 4)
                        .fill(Color.secondary.opacity(0.15))
                        .frame(height: 7)

                    RoundedRectangle(cornerRadius: 4)
                        .fill(barColor)
                        .frame(width: max(0, min(geo.size.width, geo.size.width * CGFloat(bucket.remainingFraction))), height: 7)
                        .animation(.easeInOut(duration: 0.3), value: bucket.remainingFraction)
                }
            }
            .frame(height: 7)
        }
    }
}
