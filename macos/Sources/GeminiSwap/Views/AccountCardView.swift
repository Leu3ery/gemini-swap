import SwiftUI

struct AccountCardView: View {
    let account: Account
    let isActive: Bool
    let quotaNow: Date
    let switchingText: String?
    let switchStartedAt: Date?
    let switchingDisabled: Bool
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
                    .menuIndicator(.hidden)
                    .frame(width: 20, height: 20)
                    .help("Export account config for friend")

                    if let switchingText = switchingText {
                        HStack(spacing: 6) {
                            ProgressView()
                                .controlSize(.small)
                            TimelineView(.periodic(from: switchStartedAt ?? .now, by: 1)) { context in
                                let elapsed = max(0, Int(context.date.timeIntervalSince(switchStartedAt ?? context.date)))
                                Text("\(switchingText) \(elapsed)s")
                                    .font(.system(size: 11, weight: .medium))
                                    .foregroundColor(.secondary)
                            }
                        }
                    } else if !isActive {
                        Button(action: onSwitch) {
                            Text(account.type == .oauth && account.oauth?.client != "antigravity" ? "Authorize & Switch" : "Switch")
                        }
                        .buttonStyle(.bordered)
                        .controlSize(.small)
                        .disabled(switchingDisabled)
                        .help(account.type == .oauth && account.oauth?.client != "antigravity"
                            ? "One-time Google authorization is required before Antigravity can use this account"
                            : "Switch Antigravity to this account")
                    }

                    Button(action: onRemove) {
                        Image(systemName: "trash")
                            .font(.system(size: 12))
                            .foregroundColor(.secondary)
                    }
                    .buttonStyle(.plain)
                    .disabled(switchingDisabled)
                    .help("Remove account")
                }
            }

            // Antigravity & API Quotas
            if let quota = account.lastQuota {
                if let groups = quota.groups, !groups.isEmpty {
                    Divider()
                        .padding(.vertical, 2)

                    VStack(spacing: 12) {
                        ForEach(groups) { group in
                            AntigravityQuotaGroupCard(group: group, now: quotaNow)
                        }
                    }
                } else if let buckets = quota.buckets, !buckets.isEmpty {
                    Divider()
                        .padding(.vertical, 2)

                    VStack(spacing: 8) {
                        ForEach(buckets) { bucket in
                            QuotaBucketBar(bucket: bucket, now: quotaNow)
                        }
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

struct CircularProgressRing: View {
    let fraction: Double
    var size: CGFloat = 20
    var lineWidth: CGFloat = 2.75

    var ringColor: Color {
        if fraction < 0.20 {
            return .red
        } else if fraction < 0.50 {
            return .orange
        }
        return .green
    }

    var body: some View {
        ZStack {
            Circle()
                .stroke(Color.primary.opacity(0.12), lineWidth: lineWidth)
            Circle()
                .trim(from: 0.0, to: max(0.001, min(1.0, CGFloat(fraction))))
                .stroke(
                    ringColor,
                    style: StrokeStyle(lineWidth: lineWidth, lineCap: .round)
                )
                .rotationEffect(.degrees(-90))
        }
        .frame(width: size, height: size)
    }
}

struct AntigravityQuotaGroupCard: View {
    let group: QuotaGroup
    let now: Date

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            // Group Header (e.g. Gemini Models) with info icon
            HStack(spacing: 4) {
                Text(group.displayName)
                    .font(.system(size: 11.5, weight: .semibold))
                    .foregroundColor(.primary)

                Image(systemName: "info.circle")
                    .font(.system(size: 10))
                    .foregroundColor(.secondary.opacity(0.7))
                    .help(group.description ?? group.displayName)

                Spacer()
            }

            // Group Container
            VStack(spacing: 0) {
                ForEach(Array(group.buckets.enumerated()), id: \.element.id) { index, bucket in
                    let effectiveFraction = bucket.effectiveRemainingFraction(at: now)
                    let percentage = bucket.percentage(at: now)
                    let locallyReset = bucket.hasLocallyReset(at: now)
                    if index > 0 {
                        Divider()
                            .padding(.vertical, 4)
                            .opacity(0.4)
                    }

                    HStack(alignment: .center, spacing: 10) {
                        VStack(alignment: .leading, spacing: 2) {
                            Text(bucket.displayName)
                                .font(.system(size: 11.5, weight: .medium))
                                .foregroundColor(.primary)

                            if locallyReset {
                                Text("Scheduled reset reached — shown as 100% until the next live refresh.")
                                    .font(.system(size: 10))
                                    .foregroundColor(.secondary)
                                    .fixedSize(horizontal: false, vertical: true)
                                    .lineLimit(2)
                            } else if let desc = bucket.description, !desc.isEmpty {
                                Text(desc)
                                    .font(.system(size: 10))
                                    .foregroundColor(.secondary)
                                    .fixedSize(horizontal: false, vertical: true)
                                    .lineLimit(2)
                            }
                        }

                        Spacer(minLength: 8)

                        HStack(spacing: 6) {
                            Text("\(percentage)%")
                                .font(.system(size: 11.5, weight: .bold, design: .rounded))
                                .foregroundColor(.primary)

                            CircularProgressRing(fraction: effectiveFraction)
                        }
                    }
                    .padding(.vertical, 3)
                }
            }
            .padding(.horizontal, 10)
            .padding(.vertical, 6)
            .background(
                RoundedRectangle(cornerRadius: 8)
                    .fill(Color.primary.opacity(0.025))
            )
            .overlay(
                RoundedRectangle(cornerRadius: 8)
                    .stroke(Color.primary.opacity(0.07), lineWidth: 1)
            )
        }
    }
}

struct QuotaBucketBar: View {
    let bucket: QuotaBucket
    let now: Date

    var barColor: Color {
        if bucket.percentage(at: now) < 25 {
            return .red
        } else if bucket.percentage(at: now) < 60 {
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
                if !bucket.hasLocallyReset(at: now), let rem = bucket.remainingAmount, let lim = bucket.limit, lim > 0 {
                    Text("\(bucket.percentage(at: now))% (\(rem) / \(lim))")
                        .font(.system(size: 11, weight: .medium, design: .monospaced))
                        .foregroundColor(.secondary)
                } else {
                    Text("\(bucket.percentage(at: now))%")
                        .font(.system(size: 11, weight: .medium, design: .monospaced))
                        .foregroundColor(.secondary)
                }
            }

            GeometryReader { geo in
                ZStack(alignment: .leading) {
                    Capsule()
                        .fill(Color.primary.opacity(0.08))
                        .frame(height: 5)

                    Capsule()
                        .fill(barColor)
                        .frame(width: max(0, min(geo.size.width, geo.size.width * CGFloat(bucket.effectiveRemainingFraction(at: now)))), height: 5)
                        .animation(.easeInOut(duration: 0.2), value: bucket.effectiveRemainingFraction(at: now))
                }
            }
            .frame(height: 5)
        }
    }
}
