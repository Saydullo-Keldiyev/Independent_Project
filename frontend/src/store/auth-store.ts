import { create } from 'zustand';
import api from '@/lib/api';

interface User {
  id: string;
  username: string;
  email: string;
  first_name: string;
  last_name: string;
  role: string;
}

interface AuthState {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (email: string, password: string) => Promise<void>;
  register: (data: RegisterData) => Promise<void>;
  logout: () => void;
  loadUser: () => Promise<void>;
}

interface RegisterData {
  username: string;
  email: string;
  password: string;
  first_name: string;
  last_name: string;
  role: string;
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  isAuthenticated: false,
  isLoading: true,

  login: async (email, password) => {
    const { data } = await api.post('/api/v1/auth/login', { email, password });
    const result = data.data || data;
    localStorage.setItem('access_token', result.access_token);
    localStorage.setItem('refresh_token', result.refresh_token);
    if (result.user?.id) localStorage.setItem('user_id', result.user.id);
    set({ user: result.user, isAuthenticated: true });
  },

  register: async (registerData) => {
    const { data } = await api.post('/api/v1/auth/register', registerData);
    const result = data.data || data;
    localStorage.setItem('access_token', result.access_token);
    localStorage.setItem('refresh_token', result.refresh_token);
    if (result.user?.id) localStorage.setItem('user_id', result.user.id);
    set({ user: result.user, isAuthenticated: true });
  },

  logout: () => {
    api.post('/api/v1/auth/logout').catch(() => {});
    localStorage.removeItem('access_token');
    localStorage.removeItem('refresh_token');
    set({ user: null, isAuthenticated: false });
  },

  loadUser: async () => {
    const token = localStorage.getItem('access_token');
    if (!token) {
      set({ isLoading: false, isAuthenticated: false });
      return;
    }
    // Token bor — authenticated deb hisoblaymiz
    set({ isAuthenticated: true });
    try {
      const { data } = await api.get('/api/v1/users/me');
      const userData = data.data || data;
      if (userData?.id) localStorage.setItem('user_id', userData.id);
      set({ user: userData, isAuthenticated: true, isLoading: false });
    } catch {
      // API xato berdi lekin token bor — hali ham authenticated
      set({ isLoading: false });
    }
  },
}));
