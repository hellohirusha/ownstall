import { gql } from "@apollo/client";
import { useQuery } from "@apollo/client/react";
import {
  View,
  Text,
  Image,
  ScrollView,
  StyleSheet,
  ActivityIndicator,
  TouchableOpacity,
} from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { colors } from "../theme";

const GET_PRODUCT = gql`
  query GetMobileProduct($id: ID!) {
    product(id: $id) {
      id
      name
      description
      shortDesc
      basePrice
      comparePrice
      status
      tags
      images {
        id
        url
      }
      variants {
        id
        title
        price
        stockQuantity
        isInStock
      }
    }
  }
`;

export function ProductDetailScreen({ route, navigation }: any) {
  const { productId } = route.params ?? {};
  const { data, loading } = useQuery<{ product: any }>(GET_PRODUCT, {
    variables: { id: productId },
    skip: !productId,
  });

  const product = data?.product;

  if (loading) {
    return (
      <SafeAreaView style={styles.center}>
        <ActivityIndicator size="large" color={colors.brand[600]} />
      </SafeAreaView>
    );
  }

  if (!product) {
    return (
      <SafeAreaView style={styles.center}>
        <Text style={styles.missing}>Product not found</Text>
        <TouchableOpacity onPress={() => navigation.goBack()}>
          <Text style={styles.close}>Close</Text>
        </TouchableOpacity>
      </SafeAreaView>
    );
  }

  return (
    <SafeAreaView style={styles.container}>
      <View style={styles.topBar}>
        <TouchableOpacity onPress={() => navigation.goBack()} hitSlop={12}>
          <Text style={styles.close}>Close</Text>
        </TouchableOpacity>
      </View>

      <ScrollView showsVerticalScrollIndicator={false}>
        {product.images?.[0] ? (
          <Image
            source={{ uri: product.images[0].url }}
            style={styles.hero}
            resizeMode="cover"
          />
        ) : (
          <View style={[styles.hero, styles.heroPlaceholder]} />
        )}

        <View style={styles.body}>
          <Text style={styles.name}>{product.name}</Text>

          <View style={styles.priceRow}>
            <Text style={styles.price}>${product.basePrice?.toFixed(2)}</Text>
            {product.comparePrice ? (
              <Text style={styles.compare}>${product.comparePrice.toFixed(2)}</Text>
            ) : null}
          </View>

          {product.shortDesc ? (
            <Text style={styles.shortDesc}>{product.shortDesc}</Text>
          ) : null}
          {product.description ? (
            <Text style={styles.description}>{product.description}</Text>
          ) : null}

          {product.tags?.length > 0 && (
            <View style={styles.tags}>
              {product.tags.map((tag: string) => (
                <View key={tag} style={styles.tag}>
                  <Text style={styles.tagText}>{tag}</Text>
                </View>
              ))}
            </View>
          )}

          <Text style={styles.sectionTitle}>Variants</Text>
          {(product.variants ?? []).map((v: any) => (
            <View key={v.id} style={styles.variant}>
              <View style={styles.variantLeft}>
                <Text style={styles.variantTitle}>{v.title}</Text>
                <Text
                  style={[
                    styles.stock,
                    { color: v.isInStock ? colors.success.solid : colors.danger.solid },
                  ]}
                >
                  {v.isInStock ? `${v.stockQuantity} in stock` : "Out of stock"}
                </Text>
              </View>
              <Text style={styles.variantPrice}>${v.price?.toFixed(2)}</Text>
            </View>
          ))}
        </View>
      </ScrollView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.white },
  center: {
    flex: 1,
    alignItems: "center",
    justifyContent: "center",
    backgroundColor: colors.white,
    gap: 12,
  },
  missing: { color: colors.ink[500], fontSize: 15 },
  topBar: { paddingHorizontal: 20, paddingVertical: 12, alignItems: "flex-end" },
  close: { fontSize: 15, color: colors.brand[600], fontWeight: "600" },
  hero: { width: "100%", height: 280, backgroundColor: colors.ink[100] },
  heroPlaceholder: { backgroundColor: colors.ink[100] },
  body: { padding: 20 },
  name: { fontSize: 22, fontWeight: "700", color: colors.ink[900] },
  priceRow: { flexDirection: "row", alignItems: "center", gap: 10, marginTop: 6 },
  price: { fontSize: 20, fontWeight: "700", color: colors.brand[600] },
  compare: {
    fontSize: 15,
    color: colors.ink[400],
    textDecorationLine: "line-through",
  },
  shortDesc: { fontSize: 14, color: colors.ink[600], marginTop: 12 },
  description: { fontSize: 14, color: colors.ink[500], marginTop: 8, lineHeight: 20 },
  tags: { flexDirection: "row", flexWrap: "wrap", gap: 8, marginTop: 14 },
  tag: {
    backgroundColor: colors.ink[100],
    paddingHorizontal: 10,
    paddingVertical: 4,
    borderRadius: 100,
  },
  tagText: { fontSize: 12, color: colors.ink[600] },
  sectionTitle: {
    fontSize: 15,
    fontWeight: "700",
    color: colors.ink[900],
    marginTop: 24,
    marginBottom: 10,
  },
  variant: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    backgroundColor: colors.ink[50],
    borderRadius: 12,
    padding: 14,
    marginBottom: 8,
  },
  variantLeft: { flex: 1 },
  variantTitle: { fontSize: 14, fontWeight: "500", color: colors.ink[900] },
  stock: { fontSize: 12, marginTop: 2 },
  variantPrice: { fontSize: 15, fontWeight: "700", color: colors.ink[900] },
});
