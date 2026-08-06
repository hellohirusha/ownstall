import { gql } from "@apollo/client";
import { useQuery } from "@apollo/client/react";
import { View, Text, FlatList, StyleSheet, RefreshControl } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { colors } from "../theme";

const GET_TICKETS = gql`
  query GetMobileTickets {
    tickets {
      id
      number
      subject
      status
      priority
      slaStatus
      customerEmail
      latestMessage
      unreadCount
      updatedAt
    }
  }
`;

const PRIORITY_COLOR: Record<string, string> = {
  urgent: colors.danger.solid,
  high: colors.warning.solid,
  normal: colors.ink[500],
  low: colors.info.solid,
};

const SLA_LABEL: Record<string, { text: string; color: string }> = {
  breached: { text: "SLA breached", color: colors.danger.solid },
  at_risk: { text: "SLA at risk", color: colors.warning.solid },
  ok: { text: "Within SLA", color: colors.success.solid },
  met: { text: "SLA met", color: colors.ink[400] },
};

export function SupportScreen() {
  const { data, loading, refetch } = useQuery<{ tickets: any[] }>(GET_TICKETS, {
    pollInterval: 30000,
  });
  const tickets = data?.tickets ?? [];
  const openCount = tickets.filter((t) => t.status === "open").length;

  return (
    <SafeAreaView style={styles.container}>
      <View style={styles.header}>
        <Text style={styles.title}>Support</Text>
        {openCount > 0 && (
          <View style={styles.openBadge}>
            <Text style={styles.openBadgeText}>{openCount} open</Text>
          </View>
        )}
      </View>

      <FlatList
        data={tickets}
        keyExtractor={(t) => t.id}
        contentContainerStyle={styles.list}
        refreshControl={
          <RefreshControl refreshing={loading} onRefresh={refetch} tintColor={colors.brand[600]} />
        }
        ListEmptyComponent={
          loading ? null : <Text style={styles.empty}>No tickets</Text>
        }
        renderItem={({ item }) => {
          const sla = SLA_LABEL[item.slaStatus];
          return (
            <View style={styles.card}>
              <View style={styles.cardTop}>
                <Text style={styles.number}>#{item.number}</Text>
                {item.unreadCount > 0 && <View style={styles.unreadDot} />}
              </View>

              <Text style={styles.subject} numberOfLines={1}>
                {item.subject}
              </Text>
              <Text style={styles.customer} numberOfLines={1}>
                {item.customerEmail}
              </Text>
              {item.latestMessage ? (
                <Text style={styles.preview} numberOfLines={1}>
                  {item.latestMessage}
                </Text>
              ) : null}

              <View style={styles.cardBottom}>
                <Text
                  style={[
                    styles.priority,
                    { color: PRIORITY_COLOR[item.priority] ?? colors.ink[500] },
                  ]}
                >
                  {item.priority}
                </Text>
                <Text style={styles.status}>{item.status}</Text>
                {sla ? (
                  <Text style={[styles.sla, { color: sla.color }]}>{sla.text}</Text>
                ) : null}
              </View>
            </View>
          );
        }}
      />
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.ink[50] },
  header: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    paddingHorizontal: 20,
    paddingTop: 8,
    paddingBottom: 12,
  },
  title: { fontSize: 24, fontWeight: "700", color: colors.ink[900] },
  openBadge: {
    backgroundColor: colors.brand[600],
    paddingHorizontal: 10,
    paddingVertical: 3,
    borderRadius: 100,
  },
  openBadgeText: { color: colors.white, fontSize: 12, fontWeight: "600" },
  list: { paddingHorizontal: 20, paddingBottom: 24 },
  empty: { textAlign: "center", color: colors.ink[400], marginTop: 40, fontSize: 14 },
  card: {
    backgroundColor: colors.white,
    borderRadius: 14,
    padding: 14,
    marginBottom: 10,
    borderWidth: 1,
    borderColor: colors.ink[100],
  },
  cardTop: { flexDirection: "row", alignItems: "center", marginBottom: 4 },
  number: { fontSize: 12, color: colors.ink[400], fontWeight: "600" },
  unreadDot: {
    width: 8,
    height: 8,
    borderRadius: 4,
    backgroundColor: colors.brand[600],
    marginLeft: 8,
  },
  subject: { fontSize: 14, fontWeight: "600", color: colors.ink[900] },
  customer: { fontSize: 12, color: colors.ink[500], marginTop: 2 },
  preview: { fontSize: 12, color: colors.ink[400], marginTop: 4 },
  cardBottom: { flexDirection: "row", alignItems: "center", gap: 10, marginTop: 10 },
  priority: { fontSize: 11, fontWeight: "700", textTransform: "uppercase" },
  status: { fontSize: 11, color: colors.ink[500] },
  sla: { fontSize: 11, fontWeight: "500" },
});
