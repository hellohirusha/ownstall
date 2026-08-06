# Ownstall — seller app

Expo (SDK 57) companion app for **stall owners**. Check orders, answer support
tickets and get pushed when something happens, without opening the web
dashboard.

Shoppers use the web app — this app is seller-only, and signs in with the same
credentials as `/login` on the web.

See the [root README](../README.md) for the platform overview.

> **Working on this app?** Expo changes fast. Read the exact versioned docs at
> <https://docs.expo.dev/versions/v57.0.0/> before writing code — see
> [`AGENTS.md`](AGENTS.md).

## Run it

```bash
npm install
cp .env.example .env     # then set EXPO_PUBLIC_API_URL — see below
npx expo start
```

Scan the QR code with Expo Go, or press `a` / `i` for an emulator.

| Script            | What it does                    |
| ----------------- | ------------------------------- |
| `npm start`       | Metro bundler                   |
| `npm run android` | Start and open on Android       |
| `npm run ios`     | Start and open on iOS           |
| `npx expo-doctor` | Config and dependency check     |
| `npx tsc --noEmit`| Typecheck                       |

## `EXPO_PUBLIC_API_URL` — read this before debugging a blank screen

On a physical phone, **`localhost` is the phone**, not your dev machine. With
this unset the app loads and then fails every single request, which looks like
the app itself is broken.

| Running on                | Value                              |
| ------------------------- | ---------------------------------- |
| Physical device (same Wi-Fi) | `http://<your-machine-LAN-IP>:8080` |
| Android emulator          | `http://10.0.2.2:8080`             |
| iOS simulator             | `http://localhost:8080`            |
| Deployed API              | `https://ownstall.up.railway.app`  |

Find your LAN IP with `ipconfig` (Windows) or `ifconfig | grep inet` (macOS/
Linux). `EXPO_PUBLIC_*` is inlined at build time, so restart Metro after
changing it — `npx expo start -c` to clear the cache too.

Also make sure the API is actually reachable from the phone: it must be bound
on all interfaces, and Windows Firewall must allow inbound connections on 8080.

## Screens

| Screen          | What it shows                                        |
| --------------- | ---------------------------------------------------- |
| `LoginScreen`   | Seller sign-in against `POST /api/login`             |
| `HomeScreen`    | Today's takings, recent orders, low-stock warnings    |
| `OrdersScreen`  | Order list with status filters                        |
| `ProductDetail` | One product, opened modally from Home                 |
| `SupportScreen` | Support tickets with SLA state                        |
| `ProfileScreen` | Account, push registration, sign out                  |

Offline reads are cached in `src/lib/offline.ts`, so Home and Orders still
render something on a dropped connection.

## Theme

[`src/theme.ts`](src/theme.ts) mirrors the web palette in
`frontend/tailwind.config.js` — teal `brand`, amber `accent`, `ink` neutrals.
Keep the two in step; Tailwind does not run in this runtime, so the values are
hand-copied.

Semantic colours (`success`, `warning`, `danger`, `info`) are separate from the
brand on purpose: a "paid" badge must not change meaning if the brand colour
changes.

Icons in `assets/` are generated from the same mark as the web favicon.

## Push notifications

**`expo-notifications` is imported lazily, on purpose.** SDK 53 removed remote
push from Expo Go on Android, and the module throws *while it is being
evaluated* — so a plain top-level `import` crashes the app during Metro's
module-graph evaluation, before React renders. The symptom is a red
`[runtime not ready]` screen at launch with `warnOfExpoGoPushUsage` at the top
of the stack, on the login screen, long before push is ever used.

`src/lib/notifications.ts` therefore imports it dynamically and only when
`pushSupported` is true. Do not "tidy" that into a static import.

Detecting Expo Go needs `Constants.appOwnership === "expo"`. The
non-deprecated `executionEnvironment` reports `storeClient` for both Expo Go
and a dev-client build, and a dev build *does* support push, so it cannot tell
the two apart.

Push works up to Expo's API, but delivery to a device needs an EAS project:

```bash
eas init            # writes extra.eas.projectId into app.json
eas build --profile development --platform android
```

Without a `projectId`, `registerForPushNotifications()` logs and returns `null`
rather than throwing — the app stays fully usable without push. Note that
Expo Go on Android cannot receive remote push at all since SDK 53; you need a
development build.

## Troubleshooting

| Symptom                                   | Cause                                                        |
| ----------------------------------------- | ------------------------------------------------------------ |
| Red `[runtime not ready]` at launch, `warnOfExpoGoPushUsage` in the stack | Something imports `expo-notifications` at module scope. It must stay lazy — see above |
| Loads, then every screen errors           | `EXPO_PUBLIC_API_URL` unset or unreachable from the phone     |
| Crash on launch outside Expo Go           | `react-native-worklets` missing — `npx expo install react-native-worklets` |
| Config or dependency complaints           | Run `npx expo-doctor`; it should report 20/20                |
| Stale code after editing `.env`           | Restart Metro with `npx expo start -c`                        |
