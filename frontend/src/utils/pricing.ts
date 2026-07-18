export type PricingRegion = "cn" | "global";
export type SubscriptionPlanPriceId = "basic" | "pro" | "ultra";
export type SubscriptionBillingPeriod = "monthly" | "yearly";
export type CreditRechargePackage = {
  credits: number;
  labelKey: string;
  discountKey: string;
  amounts: Record<PricingRegion, number | null>;
};

type ConfiguredAmount = number | null;

function configuredAmount(name: string): ConfiguredAmount {
  const raw = import.meta.env[name]?.trim();
  if (!raw) {
    return null;
  }

  const amount = Number(raw);
  return Number.isFinite(amount) && amount >= 0 ? amount : null;
}

const SUBSCRIPTION_PRICES: Record<
  PricingRegion,
  Record<SubscriptionPlanPriceId, Record<SubscriptionBillingPeriod, ConfiguredAmount>>
> = {
  cn: {
    basic: { monthly: configuredAmount("VITE_PRICE_CN_BASIC_MONTHLY"), yearly: configuredAmount("VITE_PRICE_CN_BASIC_YEARLY") },
    pro: { monthly: configuredAmount("VITE_PRICE_CN_PRO_MONTHLY"), yearly: configuredAmount("VITE_PRICE_CN_PRO_YEARLY") },
    ultra: { monthly: configuredAmount("VITE_PRICE_CN_ULTRA_MONTHLY"), yearly: configuredAmount("VITE_PRICE_CN_ULTRA_YEARLY") },
  },
  global: {
    basic: { monthly: configuredAmount("VITE_PRICE_GLOBAL_BASIC_MONTHLY"), yearly: configuredAmount("VITE_PRICE_GLOBAL_BASIC_YEARLY") },
    pro: { monthly: configuredAmount("VITE_PRICE_GLOBAL_PRO_MONTHLY"), yearly: configuredAmount("VITE_PRICE_GLOBAL_PRO_YEARLY") },
    ultra: { monthly: configuredAmount("VITE_PRICE_GLOBAL_ULTRA_MONTHLY"), yearly: configuredAmount("VITE_PRICE_GLOBAL_ULTRA_YEARLY") },
  },
};

function configuredCreditPackage(index: number): CreditRechargePackage | null {
  const credits = configuredAmount(`VITE_CREDIT_PACKAGE_${index}_CREDITS`);
  const cn = configuredAmount(`VITE_CREDIT_PACKAGE_${index}_CN_AMOUNT`);
  const global = configuredAmount(`VITE_CREDIT_PACKAGE_${index}_GLOBAL_AMOUNT`);

  if (!credits || (cn === null && global === null)) {
    return null;
  }

  return {
    credits,
    labelKey: `package${index}`,
    discountKey: "configured",
    amounts: { cn, global },
  };
}

export const CREDIT_RECHARGE_PACKAGES: CreditRechargePackage[] = [1, 2, 3, 4]
  .map(configuredCreditPackage)
  .filter((item): item is CreditRechargePackage => item !== null);

export function getPricingRegion(region?: string | null): PricingRegion {
  return region === "global" ? "global" : "cn";
}

export function getSubscriptionPlanAmount(
  region: PricingRegion,
  plan: SubscriptionPlanPriceId,
  billingPeriod: SubscriptionBillingPeriod,
) {
  return SUBSCRIPTION_PRICES[region][plan][billingPeriod];
}

export function getCreditRechargeAmount(region: PricingRegion, rechargePackage: CreditRechargePackage) {
  return rechargePackage.amounts[region];
}

export function formatRegionCurrency(amount: ConfiguredAmount, region: PricingRegion) {
  if (amount === null) {
    return region === "global" ? "Operator-defined" : "由部署方配置";
  }
  return `${region === "global" ? "$" : "¥"}${amount}`;
}
