"use client";

import React, { createContext, useContext, useState, useEffect } from 'react';
import { getUser } from '@/lib/auth';

const UmkmContext = createContext();

export function UmkmProvider({ children }) {
  const [umkmData, setUmkmData] = useState({
    users: {
      id: null,
      name: "UMKM SavePlate",
      email: "",
    },
    umkm_profiles: {
      business_name: "UMKM SavePlate",
      address: "",
      geo_location: "",
      rating: 0,
      keyword_safety_level: "Aman",
      level: "UMKM",
    },
    tagline: "",
    category: "Makanan Siap Saji",
    isStoreOpen: true,
    phone: "",
    description: "",
    nib: "",
    npwp: ""
  });

  useEffect(() => {
    let active = true;
    const user = getUser();
    if (!user) return;
    queueMicrotask(() => {
      if (!active) return;
      setUmkmData((prev) => ({
        ...prev,
        users: {
          id: user.id ?? prev.users.id,
          name: user.name || prev.users.name,
          email: user.email || prev.users.email,
        },
        umkm_profiles: {
          ...prev.umkm_profiles,
          business_name: user.business_name || user.name || prev.umkm_profiles.business_name,
          address: user.address || prev.umkm_profiles.address,
        },
        phone: user.phone || prev.phone,
      }));
    });
    return () => {
      active = false;
    };
  }, []);

  return (
    <UmkmContext.Provider value={{ umkmData, setUmkmData }}>
      {children}
    </UmkmContext.Provider>
  );
}

export function useUmkm() {
  const context = useContext(UmkmContext);
  if (!context) {
    throw new Error('useUmkm must be used within a UmkmProvider');
  }
  return context;
}
