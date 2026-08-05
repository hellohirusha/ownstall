import { useEffect, useState } from "react";
import { gql } from "@apollo/client";
import { useQuery } from "@apollo/client/react";
import {
  View,
  Text,
  FlatList,
  StyleSheet,
  RefreshControl,
} from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";

import { cacheOrders, getCachedOrders } from "../lib/offline";

const GET_ORDERS = gql`
  query GetMobileOrders {
    orders {
      id
      status
      total
      customerEmail
      createdAt
      items {
        productName
        quantity
      }
    }
  }
`;

const STATUS_STYLE: Record<string, { bg: string; fg: string }> = {
  pending: { bg: "#f3f4f6", fg: "#6b7280" },
  paid: { bg: "#dcfce7", fg: "#15803d" },
  processing: { bg: "#fef9c3", fg: "#a16207" },
  shipped: { bg: "#dbeafe", fg: "#1d4ed8" },
  delivered: { bg: "#dcfce7", fg: "#15803d" },
  cancelled: { bg: "#fee2e2", fg: "#b91c1c" },
  refunded: { bg: "#fee2e2", fg: "#b91c1c" },
};

export function OrdersScreen() {
  const { data, loading, error, refetch } = useQuery<{ orders: any[] }>(GET_ORDERS);
  const [cached, setCached] = useState<any[] | null>(null);

  useEffect(() => {
    if (data?.orders) void cacheOrders(data.orders);
  }, [data]);

  useEffect(() => {
    if (error && !data) {
      getCachedOrders().then((orders) => setCached(orders ?? []));
    }
  }, [error, data]);

  const usingCache = !!error && !data;
  const orders = data?.orders ?? cached ?? [];

  return (
    <SafeAreaView style={styles.container}>
      <View style={styles.header}>
        <Text style={styles.title}>Orders</Text>
        <Text style={styles.count}>{orders.length}</Text>
      </View>

      {usingCache && (
        <View style={styles.offlineBanner}>
          <Text style={styles.offlineText}>Offline — showing saved data</Text>
        </View>
      )}

      <FlatList
        data={orders}
        keyExtractor={(o) => o.id}
        contentContainerStyle={styles.list}
        refreshControl={
          <RefreshControl refreshing={loading} onRefresh={refetch} tintColor="#22c55e" />
        }
        ListEmptyComponent={
          loading ? null : <Text style={styles.empty}>No orders yet</Text>
        }
        renderItem={({ item }) => {
          const style = STATUS_STYLE[item.status] ?? STATUS_STYLE.pending;
          return (
            <View style={styles.card}>
              <View style={styles.cardTop}>
                <Text style={styles.orderId}>
                  #{item.id.slice(-6).toUpperCase()}
                </Text>
                <View style={[styles.badge, { backgroundColor: style.bg }]}>
                  <Text style={[styles.badgeText, { color: style.fg }]}>
                    {item.status}
                  </Text>
                </View>
              </View>

              <Text style={styles.customer} numberOfLines={1}>
                {item.customerEmail}
              </Text>
              <Text style={styles.items} numberOfLines={1}>
                {item.items?.[0]?.productName ?? "No items"}
                {item.items?.length > 1 ? ` +${item.items.length - 1} more` : ""}
              </Text>

              <View style={styles.cardBottom}>
                <Text style={styles.date}>
                  {new Date(item.createdAt).toLocaleDateString()}
                </Text>
                <Text style={styles.total}>${item.total?.toFixed(2)}</Text>
              </View>
            </View>
          );
        }}
      />
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: "#f9fafb" },
  header: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    paddingHorizontal: 20,
    paddingTop: 8,
    paddingBottom: 12,
  },
  title: { fontSize: 24, fontWeight: "700", color: "#111827" },
  count: { fontSize: 14, color: "#9ca3af" },
  offlineBanner: {
    marginHorizontal: 20,
    marginBottom: 12,
    backgroundColor: "#fef3c7",
    borderRadius: 10,
    paddingVertical: 8,
    paddingHorizontal: 12,
  },
  offlineText: { fontSize: 12, color: "#92400e", fontWeight: "500" },
  list: { paddingHorizontal: 20, paddingBottom: 24 },
  empty: { textAlign: "center", color: "#9ca3af", marginTop: 40, fontSize: 14 },
  card: {
    backgroundColor: "#fff",
    borderRadius: 14,
    padding: 14,
    marginBottom: 10,
    borderWidth: 1,
    borderColor: "#f3f4f6",
  },
  cardTop: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    marginBottom: 6,
  },
  orderId: { fontSize: 12, color: "#9ca3af", fontWeight: "600" },
  badge: { paddingHorizontal: 8, paddingVertical: 2, borderRadius: 100 },
  badgeText: { fontSize: 11, fontWeight: "600" },
  customer: { fontSize: 14, fontWeight: "500", color: "#111827" },
  items: { fontSize: 12, color: "#9ca3af", marginTop: 2 },
  cardBottom: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    marginTop: 10,
  },
  date: { fontSize: 12, color: "#9ca3af" },
  total: { fontSize: 15, fontWeight: "700", color: "#111827" },
});
