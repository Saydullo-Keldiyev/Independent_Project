import api from '@/lib/api';

export interface AdminUser {
  id: string;
  username: string;
  email: string;
  first_name: string;
  last_name: string;
  role: string;
  is_verified: boolean;
  is_active: boolean;
  created_at: string;
}

export interface ListUsersResponse {
  users: AdminUser[];
  total: number;
  page: number;
  limit: number;
}

export const adminService = {
  listUsers: (params?: { page?: number; limit?: number; role?: string; search?: string }) =>
    api.get('/api/v1/admin/users', { params }).then(r => r.data),

  updateRole: (userId: string, role: string) =>
    api.put(`/api/v1/admin/users/${userId}/role`, { role }).then(r => r.data),

  banUser: (userId: string) =>
    api.post(`/api/v1/admin/users/${userId}/ban`).then(r => r.data),

  unbanUser: (userId: string) =>
    api.post(`/api/v1/admin/users/${userId}/unban`).then(r => r.data),

  deleteUser: (userId: string) =>
    api.delete(`/api/v1/admin/users/${userId}`).then(r => r.data),
};
