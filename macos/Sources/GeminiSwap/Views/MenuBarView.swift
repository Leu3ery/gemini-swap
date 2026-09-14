import SwiftUI

struct MenuBarView: View {
    @ObservedObject var appState: AppState
    var onOpenWindow: () -> Void
    var onQuit: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            // Header
            HStack {
                Text("Gemini Swap")
                    .font(.system(size: 13, weight: .semibold))

                Spacer()

                Button(action: onOpenWindow) {
                    Image(systemName: "arrow.up.left.and.arrow.down.right")
                        .font(.system(size: 11))
                }
                .buttonStyle(.plain)
                .help("Open full window")
            }
            .padding(.bottom, 2)

            Divider()

            // Active Account & Quotas
            if let active = appState.activeAccount {
                VStack(alignment: .leading, spacing: 6) {
                    HStack(spacing: 6) {
                        Circle()
                            .fill(Color.green)
                            .frame(width: 7, height: 7)
                        Text(active.name)
                            .font(.system(size: 13, weight: .semibold))
                            .lineLimit(1)
                    }

                    if let email = active.email, !email.isEmpty, email != active.name {
                        Text(email)
                            .font(.system(size: 11))
                            .foregroundColor(.secondary)
                            .lineLimit(1)
                    }

                    // Mini Quota Bars
                    if let quota = active.lastQuota, !quota.allBuckets.isEmpty {
                        VStack(spacing: 6) {
                            ForEach(quota.allBuckets.prefix(4)) { bucket in
                                let percentage = bucket.percentage(at: appState.quotaClock)
                                let effectiveFraction = bucket.effectiveRemainingFraction(at: appState.quotaClock)
                                HStack {
                                    Text(bucket.displayName)
                                        .font(.system(size: 11))
                                        .lineLimit(1)
                                    Spacer()
                                    Text("\(percentage)%")
                                        .font(.system(size: 11, weight: .bold))
                                        .foregroundColor(percentage < 30 ? .red : (percentage < 60 ? .orange : .green))
                                }

                                GeometryReader { geo in
                                    ZStack(alignment: .leading) {
                                        RoundedRectangle(cornerRadius: 3)
                                            .fill(Color.secondary.opacity(0.2))
                                            .frame(height: 5)
                                        RoundedRectangle(cornerRadius: 3)
                                            .fill(percentage < 30 ? Color.red : (percentage < 60 ? Color.orange : Color.green))
                                            .frame(width: max(0, min(geo.size.width, geo.size.width * CGFloat(effectiveFraction))), height: 5)
                                    }
                                }
                                .frame(height: 5)
                            }
                        }
                        .padding(.top, 4)
                    }
                }
                .padding(10)
                .background(Color(NSColor.controlBackgroundColor).opacity(0.8))
                .cornerRadius(8)
            } else {
                Text("No accounts configured")
                    .font(.system(size: 12))
                    .foregroundColor(.secondary)
            }

            // Quick Account Switcher
            if appState.accounts.count > 1 {
                VStack(alignment: .leading, spacing: 4) {
                    Text("SWITCH ACCOUNT")
                        .font(.system(size: 10, weight: .bold))
                        .foregroundColor(.secondary)

                    VStack(spacing: 2) {
                        ForEach(appState.accounts) { acc in
                            let isCurrent = acc.id == appState.activeAccountID
                            let isSwitchTarget = acc.id == appState.switchingAccountID
                            Button(action: {
                                appState.switchAccount(to: acc.id)
                            }) {
                                HStack {
                                    if isSwitchTarget {
                                        ProgressView()
                                            .controlSize(.small)
                                            .frame(width: 12, height: 12)
                                    } else {
                                        Image(systemName: isCurrent ? "checkmark.circle.fill" : "circle")
                                            .foregroundColor(isCurrent ? .blue : .secondary)
                                            .font(.system(size: 12))
                                    }

                                    Text(acc.name)
                                        .font(.system(size: 12, weight: isCurrent ? .semibold : .regular))
                                        .lineLimit(1)

                                    Spacer()

                                    if let quota = acc.lastQuota?.allBuckets.first {
                                        Text("\(quota.percentage(at: appState.quotaClock))%")
                                            .font(.system(size: 10, weight: .medium))
                                            .foregroundColor(.secondary)
                                    }
                                }
                                .padding(.vertical, 4)
                                .padding(.horizontal, 6)
                                .contentShape(Rectangle())
                            }
                            .buttonStyle(.plain)
                            .disabled(appState.isSwitchingAccount)
                            .background(isCurrent ? Color.blue.opacity(0.1) : Color.clear)
                            .cornerRadius(5)
                        }
                    }
                }
                .padding(.top, 2)
            }

            Divider()

            // Footer Actions
            HStack {
                Button(action: {
                    appState.refreshQuotas()
                }) {
                    HStack(spacing: 4) {
                        Image(systemName: "arrow.clockwise")
                        Text("Refresh")
                    }
                    .font(.system(size: 11))
                }
                .buttonStyle(.plain)

                Spacer()

                Button(action: onOpenWindow) {
                    Text("Dashboard")
                        .font(.system(size: 11, weight: .medium))
                }
                .buttonStyle(.bordered)
                .controlSize(.small)

                Button(action: onQuit) {
                    Text("Quit")
                        .font(.system(size: 11))
                        .foregroundColor(.red)
                }
                .buttonStyle(.plain)
            }
        }
        .padding(14)
        .frame(width: 290)
    }
}
