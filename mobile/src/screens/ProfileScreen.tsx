import { useEffect, useState } from "react";
import { View, Text, TouchableOpacity, StyleSheet } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";

import { getUser, logout, AuthUser } from "../lib/auth";
import { useAuth } from "../lib/authContext";
import { API_URL } from "../lib/apollo";
import { cacheClear } from "../lib/offline";
import { colors } from "../theme";

export function ProfileScreen() {
  const { setAuthenticated } = useAuth();
  const [user, setUser] = useState<AuthUser | null>(null);

  useEffect(() => {
    getUser().then(setUser);
  }, []);

  const handleLogout = async () => {
    // Drop the offline copy too, so the next user cannot read it
    await Promise.all([logout(), cacheClear()]);
    setAuthenticated(false);
  };

  const initial = (user?.email ?? "?")[0].toUpperCase();

  return (
    <SafeAreaView style={styles.container}>
      <View style={styles.header}>
        <Text style={styles.title}>Profile</Text>
      </View>

      <View style={styles.card}>
        <View style={styles.avatarRow}>
          <View style={styles.avatar}>
            <Text style={styles.avatarText}>{initial}</Text>
          </View>
          <View style={styles.identity}>
            <Text style={styles.email} numberOfLines={1}>
              {user?.email ?? "Not signed in"}
            </Text>
            {user?.role ? <Text style={styles.role}>{user.role}</Text> : null}
          </View>
        </View>

        <View style={styles.divider} />

        <View style={styles.row}>
          <Text style={styles.rowLabel}>Store</Text>
          <Text style={styles.rowValue} numberOfLines={1}>
            {user?.tenantId ? user.tenantId.slice(0, 8) : "—"}
          </Text>
        </View>
        <View style={styles.row}>
          <Text style={styles.rowLabel}>API</Text>
          <Text style={styles.rowValue} numberOfLines={1}>
            {API_URL}
          </Text>
        </View>
      </View>

      <TouchableOpacity onPress={handleLogout} style={styles.logout} activeOpacity={0.85}>
        <Text style={styles.logoutText}>Sign out</Text>
      </TouchableOpacity>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.ink[50] },
  header: { paddingHorizontal: 20, paddingTop: 8, paddingBottom: 12 },
  title: { fontSize: 24, fontWeight: "700", color: colors.ink[900] },
  card: {
    marginHorizontal: 20,
    backgroundColor: colors.white,
    borderRadius: 16,
    padding: 16,
    borderWidth: 1,
    borderColor: colors.ink[100],
  },
  avatarRow: { flexDirection: "row", alignItems: "center" },
  avatar: {
    width: 52,
    height: 52,
    borderRadius: 26,
    backgroundColor: colors.ink[900],
    alignItems: "center",
    justifyContent: "center",
  },
  avatarText: { color: colors.white, fontSize: 20, fontWeight: "700" },
  identity: { marginLeft: 14, flex: 1 },
  email: { fontSize: 15, fontWeight: "600", color: colors.ink[900] },
  role: { fontSize: 13, color: colors.ink[500], marginTop: 2, textTransform: "capitalize" },
  divider: { height: 1, backgroundColor: colors.ink[100], marginVertical: 14 },
  row: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    paddingVertical: 6,
  },
  rowLabel: { fontSize: 13, color: colors.ink[500] },
  rowValue: { fontSize: 13, color: colors.ink[900], maxWidth: "65%" },
  logout: {
    marginHorizontal: 20,
    marginTop: 20,
    borderWidth: 1,
    borderColor: colors.danger.bg,
    backgroundColor: colors.white,
    borderRadius: 12,
    paddingVertical: 14,
    alignItems: "center",
  },
  logoutText: { color: colors.danger.solid, fontSize: 15, fontWeight: "600" },
});
