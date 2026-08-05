import { useEffect, useState } from "react";
import { gql } from "@apollo/client";
import { useQuery } from "@apollo/client/react";
import {
  View,
  Text,
  TouchableOpacity,
  StyleSheet,
  Image,
  RefreshControl,
  ScrollView,
} from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";

import {
  cacheOrders,
  cacheProducts,
  getCachedOrders,
  getCachedProducts,
} from "../lib/offline";

// The API's orders query takes no limit argument — trim client-side
const GET_HOME_DATA = gql`
  query GetHomeData {
    products(status: "active") {
      id
      name
      basePrice
      images {
        url
      }
      variants {
        isInStock
      }
    }
    orders {
      id
      status
      total
      createdAt
      items {
        productName
        quantity
      }
    }
  }
`;

export function HomeScreen({ navigation }: any) {
  const { data, loading, error, refetch } = useQuery<{
    products: any[];
    orders: any[];
  }>(GET_HOME_DATA);

  const [cached, setCached] = useState<{ products: any[]; orders: any[] } | null>(null);

  // Keep the offline copy fresh whenever a fetch succeeds
  useEffect(() => {
    if (data?.products) void cacheProducts(data.products);
    if (data?.orders) void cacheOrders(data.orders);
  }, [data]);

  // No network and nothing in the Apollo cache — fall back to disk
  useEffect(() => {
    if (error && !data) {
      Promise.all([getCachedProducts(), getCachedOrders()]).then(
        ([products, orders]) =>
          setCached({ products: products ?? [], orders: orders ?? [] }),
      );
    }
  }, [error, data]);

  const usingCache = !!error && !data;
  const products = data?.products ?? cached?.products ?? [];
  const recentOrders = (data?.orders ?? cached?.orders ?? []).slice(0, 3);

  return (
    <SafeAreaView style={styles.container}>
      <ScrollView
        refreshControl={
          <RefreshControl refreshing={loading} onRefresh={refetch} tintColor="#22c55e" />
        }
        showsVerticalScrollIndicator={false}
      >
        {/* Header */}
        <View style={styles.header}>
          <View>
            <Text style={styles.greeting}>Good morning 👋</Text>
            <Text style={styles.title}>Your store</Text>
          </View>
          <View style={styles.headerDot} />
        </View>

        {usingCache && (
          <View style={styles.offlineBanner}>
            <Text style={styles.offlineText}>
              Offline — showing saved data
            </Text>
          </View>
        )}

        {/* Recent orders */}
        {recentOrders.length > 0 && (
          <View style={styles.section}>
            <Text style={styles.sectionTitle}>Recent orders</Text>
            {recentOrders.map((order: any) => (
              <View key={order.id} style={styles.orderCard}>
                <View style={styles.orderLeft}>
                  <Text style={styles.orderItem} numberOfLines={1}>
                    {order.items?.[0]?.productName ?? "Order"}
                    {order.items?.length > 1 ? ` +${order.items.length - 1}` : ""}
                  </Text>
                  <Text style={styles.orderDate}>
                    {new Date(order.createdAt).toLocaleDateString()}
                  </Text>
                </View>
                <View style={styles.orderRight}>
                  <Text style={styles.orderTotal}>${order.total?.toFixed(2)}</Text>
                  <View
                    style={[
                      styles.statusBadge,
                      order.status === "paid" && styles.statusPaid,
                      order.status === "shipped" && styles.statusShipped,
                    ]}
                  >
                    <Text style={styles.statusText}>{order.status}</Text>
                  </View>
                </View>
              </View>
            ))}
          </View>
        )}

        {/* Products grid */}
        <View style={styles.section}>
          <Text style={styles.sectionTitle}>Products</Text>
          {products.length === 0 && !loading ? (
            <Text style={styles.empty}>No active products yet</Text>
          ) : (
            <View style={styles.productsGrid}>
              {products.map((product: any) => (
                <TouchableOpacity
                  key={product.id}
                  style={styles.productCard}
                  onPress={() =>
                    navigation.navigate("ProductDetail", { productId: product.id })
                  }
                  activeOpacity={0.8}
                >
                  <View style={styles.productImageContainer}>
                    {product.images?.[0] ? (
                      <Image
                        source={{ uri: product.images[0].url }}
                        style={styles.productImage}
                        resizeMode="cover"
                      />
                    ) : (
                      <View style={[styles.productImage, styles.productImagePlaceholder]} />
                    )}
                  </View>
                  <Text style={styles.productName} numberOfLines={1}>
                    {product.name}
                  </Text>
                  <Text style={styles.productPrice}>${product.basePrice?.toFixed(2)}</Text>
                </TouchableOpacity>
              ))}
            </View>
          )}
        </View>
      </ScrollView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: "#f9fafb" },
  header: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    paddingHorizontal: 20,
    paddingTop: 8,
    paddingBottom: 16,
  },
  greeting: { fontSize: 14, color: "#9ca3af" },
  title: { fontSize: 24, fontWeight: "700", color: "#111827", marginTop: 2 },
  headerDot: {
    width: 40,
    height: 40,
    borderRadius: 20,
    backgroundColor: "#22c55e",
    opacity: 0.15,
  },
  offlineBanner: {
    marginHorizontal: 20,
    marginBottom: 16,
    backgroundColor: "#fef3c7",
    borderRadius: 10,
    paddingVertical: 8,
    paddingHorizontal: 12,
  },
  offlineText: { fontSize: 12, color: "#92400e", fontWeight: "500" },
  section: { paddingHorizontal: 20, marginBottom: 24 },
  sectionTitle: { fontSize: 16, fontWeight: "700", color: "#111827", marginBottom: 12 },
  empty: { fontSize: 13, color: "#9ca3af" },
  orderCard: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    backgroundColor: "#fff",
    borderRadius: 12,
    padding: 14,
    marginBottom: 8,
    borderWidth: 1,
    borderColor: "#f3f4f6",
  },
  orderLeft: { flex: 1, paddingRight: 12 },
  orderItem: { fontSize: 14, fontWeight: "500", color: "#111827" },
  orderDate: { fontSize: 12, color: "#9ca3af", marginTop: 2 },
  orderRight: { alignItems: "flex-end" },
  orderTotal: { fontSize: 14, fontWeight: "700", color: "#111827" },
  statusBadge: {
    marginTop: 4,
    paddingHorizontal: 8,
    paddingVertical: 2,
    borderRadius: 100,
    backgroundColor: "#f3f4f6",
  },
  statusPaid: { backgroundColor: "#dcfce7" },
  statusShipped: { backgroundColor: "#dbeafe" },
  statusText: { fontSize: 11, fontWeight: "500", color: "#6b7280" },
  productsGrid: { flexDirection: "row", flexWrap: "wrap", gap: 12 },
  productCard: {
    width: "47%",
    backgroundColor: "#fff",
    borderRadius: 16,
    overflow: "hidden",
    borderWidth: 1,
    borderColor: "#f3f4f6",
  },
  productImageContainer: { aspectRatio: 1 },
  productImage: { width: "100%", height: "100%" },
  productImagePlaceholder: { backgroundColor: "#f3f4f6" },
  productName: {
    fontSize: 13,
    fontWeight: "500",
    color: "#111827",
    paddingHorizontal: 10,
    paddingTop: 8,
  },
  productPrice: {
    fontSize: 13,
    fontWeight: "700",
    color: "#22c55e",
    paddingHorizontal: 10,
    paddingBottom: 10,
    paddingTop: 2,
  },
});
