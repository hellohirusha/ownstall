import {
  ApolloClient,
  InMemoryCache,
  createHttpLink,
  from,
} from "@apollo/client";
import { setContext } from "@apollo/client/link/context";
import AsyncStorage from "@react-native-async-storage/async-storage";

// On a physical device "localhost" is the phone itself — point
// EXPO_PUBLIC_API_URL at your machine's LAN IP (e.g. http://192.168.1.5:8080)
export const API_URL =
  process.env.EXPO_PUBLIC_API_URL ?? "http://localhost:8080";

const httpLink = createHttpLink({
  uri: `${API_URL}/query`,
});

const authLink = setContext(async (_, { headers }) => {
  const token = await AsyncStorage.getItem("access_token");
  return {
    headers: {
      ...headers,
      authorization: token ? `Bearer ${token}` : "",
    },
  };
});

export const apolloClient = new ApolloClient({
  link: from([authLink, httpLink]),
  cache: new InMemoryCache({
    typePolicies: {
      Product: { keyFields: ["id"] },
      Order: { keyFields: ["id"] },
      Ticket: { keyFields: ["id"] },
    },
  }),
});
