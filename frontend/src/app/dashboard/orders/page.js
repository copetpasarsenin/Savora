"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

// Halaman pesanan UMKM disatukan di /pesanan agar verifikasi kode pickup melalui satu alur.
export default function DashboardOrdersRedirectPage() {
  const router = useRouter();

  useEffect(() => {
    router.replace("/pesanan");
  }, [router]);

  return (
    <div style={{ padding: "40px", color: "#6B7280" }}>
      Mengalihkan ke halaman Pesanan...
    </div>
  );
}
