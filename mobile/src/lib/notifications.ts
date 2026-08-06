import * as Device from "expo-device";
import Constants from "expo-constants";
import { Platform } from "react-native";
import AsyncStorage from "@react-native-async-storage/async-storage";

import { API_URL } from "./apollo";
import { colors } from "../theme";

// ─────────────────────────────────────────────────────────────
// expo-notifications is imported LAZILY and never at module scope.
//
// SDK 53 removed remote push from Expo Go on Android, and the module throws
// while it is being evaluated — `addPushTokenListener` calls
// `warnOfExpoGoPushUsage`, which raises. A static import therefore kills the
// app during Metro's module graph evaluation, before React renders anything:
// you get a red "[runtime not ready]" screen at launch, on the login screen,
// long before push is ever used.
//
// Deferring the import means Expo Go runs the whole app fine and simply has
// no push, which is the correct degradation.
// ─────────────────────────────────────────────────────────────

type NotificationsModule = typeof import("expo-notifications");

// `executionEnvironment` reports "storeClient" for Expo Go AND for a
// dev-client build, and a dev build *does* support push — so it cannot tell
// the two apart. `appOwnership` is "expo" only in Expo Go. It is marked
// deprecated, but it is still the only discriminator that answers the
// question we actually need answered.
const isExpoGo = Constants.appOwnership === "expo";

// The exact combination that throws on import.
export const pushSupported = !(isExpoGo && Platform.OS === "android");

let notificationsPromise: Promise<NotificationsModule> | null = null;

async function loadNotifications(): Promise<NotificationsModule | null> {
  if (!pushSupported) return null;

  if (!notificationsPromise) {
    notificationsPromise = import("expo-notifications").then(async (module) => {
      // How notifications appear while the app is in the foreground.
      // SDK 53+ replaced shouldShowAlert with shouldShowBanner/shouldShowList.
      module.setNotificationHandler({
        handleNotification: async () => ({
          shouldPlaySound: true,
          shouldSetBadge: true,
          shouldShowBanner: true,
          shouldShowList: true,
        }),
      });
      return module;
    });
  }

  try {
    return await notificationsPromise;
  } catch (e) {
    // Reset so a later attempt is not stuck on a rejected promise
    notificationsPromise = null;
    console.log("expo-notifications unavailable:", e);
    return null;
  }
}

// Request permission and get this device's Expo push token.
// Returns null when push is unavailable (simulator, denied permission,
// no EAS projectId, or Expo Go on Android — which cannot receive push
// since SDK 53).
export async function registerForPushNotifications(): Promise<string | null> {
  if (!pushSupported) {
    console.log(
      "Push is not available in Expo Go on Android since SDK 53 — " +
        "run a development build to test it",
    );
    return null;
  }

  const Notifications = await loadNotifications();
  if (!Notifications) return null;

  if (!Device.isDevice) {
    console.log("Push notifications require a physical device");
    return null;
  }

  const { status: existingStatus } = await Notifications.getPermissionsAsync();
  let finalStatus = existingStatus;

  if (existingStatus !== "granted") {
    const { status } = await Notifications.requestPermissionsAsync();
    finalStatus = status;
  }

  if (finalStatus !== "granted") {
    console.log("Push notification permission denied");
    return null;
  }

  // Android needs a channel before any notification is shown
  if (Platform.OS === "android") {
    await Notifications.setNotificationChannelAsync("default", {
      name: "Ownstall",
      importance: Notifications.AndroidImportance.MAX,
      vibrationPattern: [0, 250, 250, 250],
      lightColor: colors.brand[600],
    });
  }

  // projectId is required — EAS sets it in app config at build time
  const projectId =
    Constants.expoConfig?.extra?.eas?.projectId ??
    Constants.easConfig?.projectId;

  if (!projectId) {
    console.log(
      "No EAS projectId configured — run `eas init` before using push notifications",
    );
    return null;
  }

  try {
    const tokenData = await Notifications.getExpoPushTokenAsync({ projectId });
    return tokenData.data;
  } catch (e) {
    console.log("Failed to get Expo push token:", e);
    return null;
  }
}

// Send the push token to the backend so it can notify this device
export async function savePushToken(token: string): Promise<void> {
  const accessToken = await AsyncStorage.getItem("access_token");
  if (!accessToken) return;

  try {
    const res = await fetch(`${API_URL}/query`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${accessToken}`,
      },
      body: JSON.stringify({
        query: `
          mutation RegisterDeviceToken($token: String!, $platform: String!) {
            registerDeviceToken(token: $token, platform: $platform) { id }
          }
        `,
        variables: {
          token,
          platform: Platform.OS, // "ios" or "android"
        },
      }),
    });

    const body = await res.json();
    if (body.errors?.length) {
      console.error("Failed to register push token:", body.errors[0].message);
    }
  } catch (e) {
    console.error("Failed to register push token:", e);
  }
}

// Called after sign-in — safe to call repeatedly, and a no-op wherever push
// is unavailable. Never throws: push is not required to use the app.
export async function initNotifications(): Promise<void> {
  try {
    const token = await registerForPushNotifications();
    if (token) {
      await savePushToken(token);
      await AsyncStorage.setItem("push_token", token);
    }
  } catch (e) {
    console.log("Push setup skipped:", e);
  }
}
