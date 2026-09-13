import SwiftUI

struct ProxyLogsView: View {
    let stats: ProxyStats?

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack {
                Label("Codex & AI Proxy Integration", systemImage: "bolt.horizontal.fill")
                    .font(.system(size: 14, weight: .semibold))
                Spacer()

                HStack(spacing: 6) {
                    Circle()
                        .fill(Color.green)
                        .frame(width: 8, height: 8)
                    Text("http://127.0.0.1:8045/v1")
                        .font(.system(size: 11, design: .monospaced))
                        .foregroundColor(.secondary)
                }
                .padding(.horizontal, 8)
                .padding(.vertical, 4)
                .background(Color.secondary.opacity(0.1))
                .cornerRadius(6)
            }

            HStack(spacing: 20) {
                StatBadge(title: "Total Requests", value: "\(stats?.totalRequests ?? 0)", icon: "arrow.up.arrow.down")
                StatBadge(title: "Auto Rotations", value: "\(stats?.rotations ?? 0)", icon: "arrow.triangle.2.circlepath")
                StatBadge(title: "Proxy Account", value: stats?.activeAccount ?? "None", icon: "person.crop.circle")
            }

            if let logs = stats?.recentLogs, !logs.isEmpty {
                VStack(alignment: .leading, spacing: 6) {
                    Text("Recent Activity (Codex / CLI)")
                        .font(.system(size: 11, weight: .semibold))
                        .foregroundColor(.secondary)

                    ScrollView {
                        VStack(spacing: 4) {
                            ForEach(logs.suffix(6).reversed()) { log in
                                HStack(spacing: 8) {
                                    Text(log.method)
                                        .font(.system(size: 10, weight: .bold, design: .monospaced))
                                        .foregroundColor(.blue)

                                    Text(log.path)
                                        .font(.system(size: 11, design: .monospaced))
                                        .lineLimit(1)

                                    Spacer()

                                    if log.rotated == true {
                                        Text("ROTATED")
                                            .font(.system(size: 9, weight: .bold))
                                            .foregroundColor(.orange)
                                            .padding(.horizontal, 4)
                                            .padding(.vertical, 1)
                                            .background(Color.orange.opacity(0.2))
                                            .cornerRadius(3)
                                    }

                                    Text("\(log.statusCode)")
                                        .font(.system(size: 10, weight: .bold, design: .monospaced))
                                        .foregroundColor(log.statusCode < 300 ? .green : .red)

                                    Text("\(log.durationMs)ms")
                                        .font(.system(size: 10, design: .monospaced))
                                        .foregroundColor(.secondary)
                                }
                                .padding(.vertical, 3)
                                .padding(.horizontal, 8)
                                .background(Color(NSColor.controlBackgroundColor).opacity(0.6))
                                .cornerRadius(4)
                            }
                        }
                    }
                    .frame(maxHeight: 120)
                }
            } else {
                Text("No proxy requests logged yet. Use 'gemini-swap proxy' or point Codex to http://127.0.0.1:8045/v1 to see live requests.")
                    .font(.system(size: 11))
                    .foregroundColor(.secondary)
                    .italic()
            }
        }
        .padding(16)
        .background(
            RoundedRectangle(cornerRadius: 12)
                .fill(Color(NSColor.controlBackgroundColor).opacity(0.5))
        )
        .overlay(
            RoundedRectangle(cornerRadius: 12)
                .stroke(Color.gray.opacity(0.15), lineWidth: 1)
        )
    }
}

struct StatBadge: View {
    let title: String
    let value: String
    let icon: String

    var body: some View {
        HStack(spacing: 8) {
            Image(systemName: icon)
                .foregroundColor(.blue)
                .font(.system(size: 14))

            VStack(alignment: .leading, spacing: 1) {
                Text(title)
                    .font(.system(size: 10))
                    .foregroundColor(.secondary)
                Text(value)
                    .font(.system(size: 13, weight: .semibold))
                    .lineLimit(1)
            }
        }
    }
}
