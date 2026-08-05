import AsyncStorage from "@react-native-async-storage/async-storage";

const CACHE_PREFIX = "cache:";
const DEFAULT_TTL_MS = 5 * 60 * 1000; // 5 minutes

interface CacheEntry<T> {
  data: T;
  expiresAt: number;
}

// Store data in AsyncStorage with TTL
export async function cacheSet<T>(key: string, data: T, ttlMs = DEFAULT_TTL_MS): Promise<void> {
  const entry: CacheEntry<T> = {
    data,
    expiresAt: Date.now() + ttlMs,
  };
  await AsyncStorage.setItem(CACHE_PREFIX + key, JSON.stringify(entry));
}

// Retrieve cached data (returns null if expired, missing or unreadable)
export async function cacheGet<T>(key: string): Promise<T | null> {
  const raw = await AsyncStorage.getItem(CACHE_PREFIX + key);
  if (!raw) return null;

  let entry: CacheEntry<T>;
  try {
    entry = JSON.parse(raw);
  } catch {
    // Corrupted entry — drop it rather than crashing the screen
    await AsyncStorage.removeItem(CACHE_PREFIX + key);
    return null;
  }

  if (Date.now() > entry.expiresAt) {
    await AsyncStorage.removeItem(CACHE_PREFIX + key);
    return null;
  }

  return entry.data;
}

// Drop every cached entry (used on sign-out)
export async function cacheClear(): Promise<void> {
  const keys = await AsyncStorage.getAllKeys();
  const ours = keys.filter((k) => k.startsWith(CACHE_PREFIX));
  if (ours.length > 0) {
    await AsyncStorage.multiRemove(ours);
  }
}

// Cache products for offline viewing
export async function cacheProducts(products: any[]): Promise<void> {
  await cacheSet("products", products, 10 * 60 * 1000); // 10 min TTL
}

export async function getCachedProducts(): Promise<any[] | null> {
  return cacheGet<any[]>("products");
}

// Cache orders
export async function cacheOrders(orders: any[]): Promise<void> {
  await cacheSet("orders", orders, 2 * 60 * 1000); // 2 min TTL
}

export async function getCachedOrders(): Promise<any[] | null> {
  return cacheGet<any[]>("orders");
}
