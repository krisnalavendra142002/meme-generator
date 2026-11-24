"use client";

// Halaman simulasi pembayaran (mock payment gateway).
// Di produksi halaman ini digantikan halaman payment gateway sungguhan
// yang memanggil webhook /confirm dengan verifikasi signature.

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { useConfirmPayment } from "@/lib/queries";
import { describeError } from "@/lib/errors";

export default function CheckoutPage() {
  const { reference } = useParams<{ reference: string }>();
  const router = useRouter();
  const confirm = useConfirmPayment();

  const onPay = async () => {
    try {
      await confirm.mutateAsync(reference);
      setTimeout(() => router.push("/pricing"), 1200);
    } catch {
      // Error ditangani dan ditampilkan melalui confirm.isError & describeError
    }
  };

  return (
    <main className="mx-auto flex min-h-[70vh] max-w-md items-center p-8">
      <Card className="w-full">
        <CardHeader>
          <CardTitle>Simulasi Pembayaran</CardTitle>
          <CardDescription>
            Payment gateway mock — tidak ada uang sungguhan.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex justify-between text-sm">
            <span className="text-muted-foreground">Referensi</span>
            <span className="font-mono">{reference}</span>
          </div>
          {confirm.data ? (
            <p className="text-center text-sm font-medium text-green-600">
              ✅ Pembayaran berhasil! Langganan Anda aktif. Mengalihkan…
            </p>
          ) : null}
          {confirm.isError ? (
            <p className="text-center text-sm font-medium text-destructive">
              {describeError(confirm.error)}
            </p>
          ) : null}
        </CardContent>
        <CardFooter className="flex gap-2">
          <Button
            className="flex-1"
            onClick={onPay}
            disabled={confirm.isPending || !!confirm.data}
          >
            {confirm.isPending
              ? "Memproses Pembayaran..."
              : confirm.data
              ? "Lunas"
              : "Bayar Sekarang (Simulasi)"}
          </Button>
          <Button asChild variant="ghost" disabled={confirm.isPending}>
            <Link href="/pricing">Batal</Link>
          </Button>
        </CardFooter>
      </Card>
    </main>
  );
}
