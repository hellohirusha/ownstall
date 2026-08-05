import AsyncStorage from "@react-native-async-storage/async-storage";
import { API_URL } from "./apollo";

export interface AuthUser {
  id: string;
  email: string;
  tenantId: string;
  role: string;
}

// The API exposes auth under /api (not /auth)
export async function login(email: string, password: string): Promise<AuthUser> {
  const res = await fetch(`${API_URL}/api/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  });

  const data = await res.json();

  if (!res.ok) {
    throw new Error(data.error ?? "Login failed");
  }

  await AsyncStorage.multiSet([
    ["access_token", data.access_token],
    ["refresh_token", data.refresh_token],
    ["user", JSON.stringify(data.user)],
  ]);

  return data.user;
}

export async function logout(): Promise<void> {
  await AsyncStorage.multiRemove(["access_token", "refresh_token", "user"]);
}

export async function getUser(): Promise<AuthUser | null> {
  const userStr = await AsyncStorage.getItem("user");
  if (!userStr) return null;
  try {
    return JSON.parse(userStr);
  } catch {
    return null;
  }
}

export async function isAuthenticated(): Promise<boolean> {
  const token = await AsyncStorage.getItem("access_token");
  return !!token;
}
