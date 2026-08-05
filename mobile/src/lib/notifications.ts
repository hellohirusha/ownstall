import * as Notifications from "expo-notifications";
import * as Device from "expo-device";
import Constants from "expo-constants";
import { Platform } from "react-native";
import AsyncStorage from "@react-native-async-storage/async-storage";

import { API_URL } from "./apollo";

// How notifications appear while the app is in the foreground.
// SDK 53+ replaced shouldShowAlert with shouldShowBanner/shouldShowList.
Notifications.setNotificationHandler({
  handleNotification: async () => ({
    shouldPlaySound: true,
    shouldSetBadge: true,
    shouldShowBanner: true,
    shouldShowList: true,
  }),
});

// Request permission and get this device's Expo push token.
// Returns null when push is unavailable (simulator, denied permission,
// or Expo Go on Android — which cannot receive push since SDK 53).
export async function registerForPushNotifications(): Promise<string | null> {
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
      lightColor: "#22c55e",
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

// Called after sign-in — safe to call repeatedly
export async function initNotifications(): Promise<void> {
  const token = await registerForPushNotifications();
  if (token) {
    await savePushToken(token);
    await AsyncStorage.setItem("push_token", token);
  }
}
