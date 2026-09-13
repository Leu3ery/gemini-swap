import SwiftUI

struct AccountCardView: View {
    let account: Account
    let isActive: Bool
    let onSwitch: () -> Void
    let onRemove: () -> Void
    let onExportFile: () -> Void
    let onCopyShareCode: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            // Header
            HStack(alignment: .center, spacing: 10) {
                // Subtle Type Icon
                Image(systemName: account.type == .oauth ? "person.crop.circle" : "key")
                    .font(.system(size: 20))
                    .foregroundColor(isActive ? .blue : .secondary)
                    .frame(width: 26, height: 26)

                VStack(alignment: .leading, spacing: 2) {
                    HStack(spacing: 8) {
                        Text(account.name)
                            .font(.system(size: 13, weight: .semibold))
                            .lineLimit(1)

                        if isActive {
                            HStack(spacing: 3) {
                                Circle()
                                    .fill(Color.green)
                                    .frame(width: 6, height: 6)
                                Text("Active")
                                    .font(.system(size: 11, weight: .medium))
                                    .foregroundColor(.green)
                            }
                        }

                        Text(account.type == .oauth ? "OAuth" : "API Key")
                            .font(.system(size: 10))
                            .foregroundColor(.secondary)
                            .padding(.horizontal, 5)
                            .padding(.vertical, 1.5)
                            .background(Color.primary.opacity(0.06))
                            .cornerRadius(4)
                    }

                    // Only show email if it is different from the account name
                    if let email = account.email, !email.isEmpty, email != account.name {
                        Text(email)
                            .font(.system(size: 11))
                            .foregroundColor(.secondary)
                            .lineLimit(1)
                    }
                }

                Spacer()

                // Actions
                HStack(spacing: 8) {
                    // Export Menu
                    Menu {
                        Button(action: onExportFile) {
                            Label("Save Config File (.json)...", systemImage: "arrow.down.doc")
                        }
                        Button(action: onCopyShareCode) {
                            Label("Copy Share Code", systemImage: "doc.on.doc")
                        }
                    } label: {
                        Image(systemName: "square.and.arrow.up")
                            .font(.system(size: 12))
                            .foregroundColor(.secondary)
                    }
                    .menuStyle(.borderlessButton)
                    .frame(width: 20, height: 20)
                    .help("Export account config for friend")

                    if !isActive {
                        Button(action: onSwitch) {
                            Text("Switch")
                        }
                        .buttonStyle(.bordered)
                        .controlSize(.small)
                    }

                    Button(action: onRemove) {
                        Image(systemName: "trash")
                            .font(.system(size: 12))
                            .foregroundColor(.secondary)
                    }
                    .buttonStyle(.plain)
                    .help("Remove account")
                }
            }

            // Quotas
            if let quota = account.lastQuota, !quota.buckets.isEmpty {
                Divider()
                    .padding(.vertical, 1)

                VStack(spacing: 8) {
                    ForEach(quota.buckets) { bucket in
                        QuotaBucketBar(bucket: bucket)
                    }
                }
            }
        }
        .padding(14)
        .background(
            RoundedRectangle(cornerRadius: 10)
                .fill(Color(NSColor.controlBackgroundColor))
        )
        .overlay(
            RoundedRectangle(cornerRadius: 10)
                .stroke(isActive ? Color.blue.opacity(0.35) : Color.primary.opacity(0.08), lineWidth: 1)
        )
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
                    .font(.system(size: 11, weight: .regular))
                    .foregroundColor(.primary)
                Spacer()
                Text("\(bucket.percentage)% (\(bucket.remainingAmount) / \(bucket.limit))")
                    .font(.system(size: 11, weight: .medium, design: .monospaced))
                    .foregroundColor(.secondary)
            }

            GeometryReader { geo in
                ZStack(alignment: .leading) {
                    Capsule()
                        .fill(Color.primary.opacity(0.08))
                        .frame(height: 5)

                    Capsule()
                        .fill(barColor)
                        .frame(width: max(0, min(geo.size.width, geo.size.width * CGFloat(bucket.remainingFraction))), height: 5)
                        .animation(.easeInOut(duration: 0.2), value: bucket.remainingFraction)
                }
            }
            .frame(height: 5)
        }
    }
}
