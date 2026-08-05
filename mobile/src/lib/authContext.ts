import { createContext, useContext } from "react";

// Lets the login and profile screens flip the app between the
// authenticated and unauthenticated navigators.
export const AuthContext = createContext<{
  setAuthenticated: (value: boolean) => void;
}>({ setAuthenticated: () => {} });

export const useAuth = () => useContext(AuthContext);
