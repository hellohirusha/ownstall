import { useCallback, useEffect, useMemo, useState } from "react";
import { ApolloProvider } from "@apollo/client/react";
import { NavigationContainer } from "@react-navigation/native";
import { createBottomTabNavigator } from "@react-navigation/bottom-tabs";
import { createStackNavigator } from "@react-navigation/stack";
import { StatusBar } from "expo-status-bar";
import { ActivityIndicator, View } from "react-native";
import { GestureHandlerRootView } from "react-native-gesture-handler";
import { SafeAreaProvider } from "react-native-safe-area-context";
import { Home, ShoppingBag, MessageSquare, User } from "lucide-react-native";

import { apolloClient } from "./src/lib/apollo";
import { isAuthenticated } from "./src/lib/auth";
import { AuthContext } from "./src/lib/authContext";
import { initNotifications } from "./src/lib/notifications";

import { LoginScreen } from "./src/screens/LoginScreen";
import { HomeScreen } from "./src/screens/HomeScreen";
import { OrdersScreen } from "./src/screens/OrdersScreen";
import { SupportScreen } from "./src/screens/SupportScreen";
import { ProfileScreen } from "./src/screens/ProfileScreen";
import { ProductDetailScreen } from "./src/screens/ProductDetailScreen";
import { colors } from "./src/theme";

const Tab = createBottomTabNavigator();
const Stack = createStackNavigator();

function MainTabs() {
  return (
    <Tab.Navigator
      screenOptions={{
        headerShown: false,
        tabBarActiveTintColor: colors.brand[600],
        tabBarInactiveTintColor: colors.ink[400],
        tabBarStyle: {
          borderTopWidth: 1,
          borderTopColor: colors.ink[100],
          height: 84,
          paddingBottom: 24,
          paddingTop: 8,
        },
      }}
    >
      <Tab.Screen
        name="Home"
        component={HomeScreen}
        options={{
          tabBarIcon: ({ color, size }) => <Home size={size} color={color} />,
        }}
      />
      <Tab.Screen
        name="Orders"
        component={OrdersScreen}
        options={{
          tabBarIcon: ({ color, size }) => <ShoppingBag size={size} color={color} />,
        }}
      />
      <Tab.Screen
        name="Support"
        component={SupportScreen}
        options={{
          tabBarIcon: ({ color, size }) => <MessageSquare size={size} color={color} />,
        }}
      />
      <Tab.Screen
        name="Profile"
        component={ProfileScreen}
        options={{
          tabBarIcon: ({ color, size }) => <User size={size} color={color} />,
        }}
      />
    </Tab.Navigator>
  );
}

function AppStack() {
  return (
    <Stack.Navigator screenOptions={{ headerShown: false }}>
      <Stack.Screen name="Main" component={MainTabs} />
      <Stack.Screen
        name="ProductDetail"
        component={ProductDetailScreen}
        options={{ presentation: "modal" }}
      />
    </Stack.Navigator>
  );
}

function AuthStack() {
  return (
    <Stack.Navigator screenOptions={{ headerShown: false }}>
      <Stack.Screen name="Login" component={LoginScreen} />
    </Stack.Navigator>
  );
}

export default function App() {
  const [loading, setLoading] = useState(true);
  const [authenticated, setAuthenticated] = useState(false);

  useEffect(() => {
    isAuthenticated().then((auth) => {
      setAuthenticated(auth);
      setLoading(false);
    });
  }, []);

  // Register this device for push once we have a session. Failures are
  // logged inside initNotifications — push is never required to use the app.
  useEffect(() => {
    if (authenticated) {
      void initNotifications();
    }
  }, [authenticated]);

  // Clear the cache on sign-out so the next user never sees stale data
  const handleAuthChange = useCallback((value: boolean) => {
    setAuthenticated(value);
    if (!value) {
      void apolloClient.clearStore();
    }
  }, []);

  const authValue = useMemo(
    () => ({ setAuthenticated: handleAuthChange }),
    [handleAuthChange],
  );

  if (loading) {
    return (
      <View style={{ flex: 1, alignItems: "center", justifyContent: "center" }}>
        <ActivityIndicator size="large" color={colors.brand[600]} />
      </View>
    );
  }

  return (
    <GestureHandlerRootView style={{ flex: 1 }}>
      <SafeAreaProvider>
        <ApolloProvider client={apolloClient}>
          <AuthContext.Provider value={authValue}>
            <NavigationContainer>
              <StatusBar style="dark" />
              {authenticated ? <AppStack /> : <AuthStack />}
            </NavigationContainer>
          </AuthContext.Provider>
        </ApolloProvider>
      </SafeAreaProvider>
    </GestureHandlerRootView>
  );
}
