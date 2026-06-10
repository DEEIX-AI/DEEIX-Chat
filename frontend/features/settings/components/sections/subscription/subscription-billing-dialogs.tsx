"use client";

import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { SpinnerLabel } from "@/components/ui/spinner";

type PaymentProvider = "stripe" | "epay";

type EPayTypeOption = {
  name: string;
  type: string;
};

function resolveEPayTypeLabel(type: string, labels: { alipay: string; wxpay: string; qqpay: string; custom: (type: string) => string }): string {
  if (type === "alipay") return labels.alipay;
  if (type === "wxpay") return labels.wxpay;
  if (type === "qqpay") return labels.qqpay;
  return labels.custom(type);
}

type TopUpDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  amount: string;
  currentBalance: string;
  billingLoading: boolean;
  topUpLoading: boolean;
  paymentDisabled: boolean;
  paymentProviders: string[];
  selectedPaymentProvider: PaymentProvider;
  selectedEPayType: string;
  epayTypes: EPayTypeOption[];
  epayLabels: {
    alipay: string;
    wxpay: string;
    qqpay: string;
    custom: (type: string) => string;
  };
  onAmountChange: (value: string) => void;
  onPaymentProviderChange: (provider: PaymentProvider) => void;
  onEPayTypeChange: (type: string) => void;
  onSubmit: () => void;
};

export function TopUpDialog({
  open,
  onOpenChange,
  amount,
  currentBalance,
  billingLoading,
  topUpLoading,
  paymentDisabled,
  paymentProviders,
  selectedPaymentProvider,
  selectedEPayType,
  epayTypes,
  epayLabels,
  onAmountChange,
  onPaymentProviderChange,
  onEPayTypeChange,
  onSubmit,
}: TopUpDialogProps) {
  const t = useTranslations("settings.subscriptionPage");

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[420px]">
        <DialogHeader>
          <DialogTitle>{t("topUp.title")}</DialogTitle>
          <DialogDescription>{t("topUp.description")}</DialogDescription>
        </DialogHeader>

        <div className="space-y-1.5">
          <div className="flex items-center justify-between gap-3">
            <p className="text-xs text-muted-foreground">{t("topUp.amount")}</p>
            <p className="truncate text-xs text-muted-foreground tabular-nums">
              {t("topUp.currentBalance", { value: currentBalance })}
            </p>
          </div>
          <div className="relative">
            <span className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-xs text-muted-foreground">$</span>
            <Input
              value={amount}
              type="number"
              min="0"
              step="0.01"
              className="pl-7"
              onChange={(event) => onAmountChange(event.target.value)}
              disabled={billingLoading || topUpLoading || paymentDisabled}
              aria-label={t("topUp.amountAria")}
            />
          </div>
        </div>

        {!paymentDisabled ? (
          <div className="space-y-2">
            <p className="text-xs text-muted-foreground">{t("payment.method")}</p>
            <div className="grid grid-cols-2 gap-2">
              {paymentProviders.includes("stripe") ? (
                <button
                  type="button"
                  className={`flex h-9 items-center justify-center rounded-md border px-2 text-xs ${
                    selectedPaymentProvider === "stripe" ? "border-foreground bg-muted/25 font-medium" : "border-border bg-transparent text-muted-foreground"
                  }`}
                  disabled={billingLoading || topUpLoading || paymentDisabled}
                  onClick={() => onPaymentProviderChange("stripe")}
                >
                  Stripe
                </button>
              ) : null}
              {paymentProviders.includes("epay")
                ? epayTypes.map((item) => {
                  const selected = selectedPaymentProvider === "epay" && selectedEPayType === item.type;
                  return (
                    <button
                      key={item.type}
                      type="button"
                      className={`flex h-9 items-center justify-center rounded-md border px-2 text-xs ${
                        selected ? "border-foreground bg-muted/25 font-medium" : "border-border bg-transparent text-muted-foreground"
                      }`}
                      disabled={billingLoading || topUpLoading || paymentDisabled}
                      onClick={() => {
                        onPaymentProviderChange("epay");
                        onEPayTypeChange(item.type);
                      }}
                    >
                      {item.name || resolveEPayTypeLabel(item.type, epayLabels)}
                    </button>
                  );
                })
                : null}
            </div>
          </div>
        ) : null}

        <DialogFooter>
          <Button type="button" variant="ghost" onClick={() => onOpenChange(false)} disabled={topUpLoading}>
            {t("actions.cancel")}
          </Button>
          <Button type="button" disabled={billingLoading || topUpLoading || paymentDisabled} onClick={onSubmit}>
            {topUpLoading ? <SpinnerLabel>{t("actions.processing")}</SpinnerLabel> : t("topUp.confirm")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

type RedemptionDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  code: string;
  billingLoading: boolean;
  redemptionLoading: boolean;
  onCodeChange: (value: string) => void;
  onSubmit: () => void;
};

export function RedemptionDialog({
  open,
  onOpenChange,
  code,
  billingLoading,
  redemptionLoading,
  onCodeChange,
  onSubmit,
}: RedemptionDialogProps) {
  const t = useTranslations("settings.subscriptionPage");

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[420px]">
        <DialogHeader>
          <DialogTitle>{t("redemption.title")}</DialogTitle>
          <DialogDescription>{t("redemption.description")}</DialogDescription>
        </DialogHeader>
        <div className="space-y-1">
          <p className="text-xs text-muted-foreground">{t("redemption.code")}</p>
          <Input
            value={code}
            autoComplete="off"
            className="font-mono"
            disabled={billingLoading || redemptionLoading}
            onChange={(event) => onCodeChange(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter") {
                event.preventDefault();
                onSubmit();
              }
            }}
            aria-label={t("redemption.code")}
          />
        </div>
        <DialogFooter>
          <Button type="button" variant="ghost" onClick={() => onOpenChange(false)} disabled={redemptionLoading}>
            {t("actions.cancel")}
          </Button>
          <Button type="button" disabled={billingLoading || redemptionLoading} onClick={onSubmit}>
            {redemptionLoading ? <SpinnerLabel>{t("actions.processing")}</SpinnerLabel> : t("redemption.confirm")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
