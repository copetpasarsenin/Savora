"use client";

import React, { createContext, useContext, useState, useEffect, useCallback } from 'react';
import { fetchNotifications, fetchUnreadCount, markAsRead, markAllAsRead } from '@/lib/notifications';
import { isAuthenticated } from '@/lib/auth';

const NotificationContext = createContext();

export function NotificationProvider({ children, userId = 1, userRole = 'umkm' }) {
  const [notifications, setNotifications] = useState([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);

  const loadNotifications = useCallback(async () => {
    if (!isAuthenticated()) return;
    try {
      setLoading(true);
      const [notifs, count] = await Promise.all([
        fetchNotifications(userId, userRole),
        fetchUnreadCount(userId, userRole),
      ]);
      setNotifications(notifs || []);
      setUnreadCount(count || 0);
      setError(null);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [userId, userRole]);

  // Fetch notifikasi & unread count saat mount (hanya jika sudah login)
  useEffect(() => {
    if (!isAuthenticated()) return;
    let active = true;
    queueMicrotask(() => {
      if (active) loadNotifications();
    });
    // Poll setiap 30 detik untuk notifikasi baru
    const interval = setInterval(loadNotifications, 30000);
    return () => {
      active = false;
      clearInterval(interval);
    };
  }, [loadNotifications]);

  const handleMarkAsRead = async (notificationId) => {
    await markAsRead(notificationId);
    // Update local state
    setNotifications(prev =>
      prev.map(n => n.id === notificationId ? { ...n, is_read: true } : n)
    );
    setUnreadCount(prev => Math.max(0, prev - 1));
  };

  const handleMarkAllAsRead = async () => {
    await markAllAsRead(userId, userRole);
    // Update local state
    setNotifications(prev => prev.map(n => ({ ...n, is_read: true })));
    setUnreadCount(0);
  };

  const value = {
    notifications,
    unreadCount,
    loading,
    error,
    markAsRead: handleMarkAsRead,
    markAllAsRead: handleMarkAllAsRead,
    refresh: loadNotifications,
  };

  return (
    <NotificationContext.Provider value={value}>
      {children}
    </NotificationContext.Provider>
  );
}

export function useNotifications() {
  const context = useContext(NotificationContext);
  if (!context) {
    throw new Error('useNotifications must be used within NotificationProvider');
  }
  return context;
}
