// The version string recorded against a seller's acceptance at signup. Bump it
// whenever the seller terms change materially — existing acceptances then no
// longer match the current version, which is what lets us ask sellers to
// re-accept rather than silently binding them to terms they never saw.
export const TERMS_VERSION = "2026-08-06";

export const TERMS_EFFECTIVE = "6 August 2026";
export const PRIVACY_EFFECTIVE = "6 August 2026";

export interface LegalSection {
  heading: string;
  body: string[];
}

export const TERMS_SECTIONS: LegalSection[] = [
  {
    heading: "1. Who these terms are for",
    body: [
      "These terms cover two groups. Sellers are people or businesses who open a stall on Ownstall and sell through it. Buyers are people who browse stalls and place orders, whether or not they hold an Ownstall account.",
      "Opening a stall means accepting these terms in full. Placing an order means accepting the parts that apply to buyers.",
    ],
  },
  {
    heading: "2. Opening a stall",
    body: [
      "You must give accurate information when you open a stall, including a name that you have the right to use. Stall names that impersonate another business or person will be rejected.",
      "Every stall is reviewed before it becomes publicly visible. Until it is approved, your stall does not appear in search and cannot take orders. We do not guarantee that any stall will be approved, and we may ask for more information before deciding.",
      "You are responsible for everything published on your stall: product listings, images, descriptions, prices and stock levels.",
    ],
  },
  {
    heading: "3. What you may not sell",
    body: [
      "You may not list illegal goods, counterfeit or infringing items, stolen property, weapons, regulated drugs, live animals, or anything you are not legally permitted to sell in your jurisdiction.",
      "You may not use a stall to collect payment for goods you do not intend to ship, or to route buyers off the platform in order to avoid these terms.",
    ],
  },
  {
    heading: "4. Approval, restriction and suspension",
    body: [
      "We may restrict a stall — for example by preventing it publishing new products or accepting new orders — where we have reasonable grounds to believe these terms have been broken, or where buyer complaints indicate a serious problem.",
      "We may suspend a stall outright for repeated or serious breaches, for fraud, or where required by law. Where practical we will tell you why and give you a chance to respond, but we may act first where buyers are at risk.",
      "You can close your stall at any time. Closing it does not cancel obligations for orders already placed.",
    ],
  },
  {
    heading: "5. Orders and payment",
    body: [
      "The contract of sale for an order is between the buyer and the seller. Ownstall provides the storefront, the checkout and the surrounding tooling; it is not the seller of the goods.",
      "Payments are processed by Stripe. Ownstall does not receive or store full card details. Sellers are responsible for having a valid payment account in order to be paid out.",
      "Sellers set their own prices, shipping arrangements, refund and return policies, and are responsible for honouring them.",
    ],
  },
  {
    heading: "6. Buyer accounts",
    body: [
      "You can buy as a guest or with an account. An account stores your order history and saves you re-entering details; it is not required to place an order.",
      "Keep your password to yourself. You are responsible for activity under your account until you tell us it has been compromised.",
    ],
  },
  {
    heading: "7. Content and intellectual property",
    body: [
      "You keep ownership of the content you upload. By uploading it you give Ownstall permission to host, resize and display it for the purpose of running the marketplace and showing your stall to shoppers.",
      "If you believe content on Ownstall infringes your rights, contact us with enough detail to identify it and we will review it.",
    ],
  },
  {
    heading: "8. Availability and liability",
    body: [
      "Ownstall is provided as-is. We work to keep it available but do not promise uninterrupted service, and we are not liable for losses caused by downtime, data loss, or the acts of any seller or buyer.",
      "Nothing here limits liability that cannot legally be limited.",
    ],
  },
  {
    heading: "9. Changes",
    body: [
      "We may update these terms. Material changes bump the version recorded against your acceptance, and sellers will be asked to accept the new version before continuing to use their stall.",
    ],
  },
];

export const PRIVACY_SECTIONS: LegalSection[] = [
  {
    heading: "1. What we collect",
    body: [
      "For sellers: the name and email address you sign up with, your stall's name and subdomain, and the products, orders and support conversations that belong to your stall.",
      "For buyers with an account: your email address, your name if you give it, and your order history.",
      "For guest buyers: the email address and delivery details you give at checkout, so the order can be confirmed and shipped.",
      "For everyone: basic technical data such as IP address and browser type, which we use to keep the service secure and to diagnose faults.",
    ],
  },
  {
    heading: "2. What we do with it",
    body: [
      "We use your data to run the marketplace: showing stalls to shoppers, processing orders, sending order confirmations and support replies, reviewing stalls before approval, and investigating abuse.",
      "We do not sell your personal data.",
    ],
  },
  {
    heading: "3. Who else sees it",
    body: [
      "When you place an order, the seller of that stall receives the details they need to fulfil it — your name, delivery address and contact email.",
      "We use third parties to operate the service: Stripe for payments, Resend for transactional email, Cloudinary for image hosting, and our hosting and error-monitoring providers. They only receive what they need to perform their function.",
      "We may disclose data where the law requires it.",
    ],
  },
  {
    heading: "4. Payment details",
    body: [
      "Card details are entered on Stripe's hosted checkout and never reach Ownstall's servers. We store only the identifiers Stripe gives us to reference a payment.",
    ],
  },
  {
    heading: "5. How long we keep it",
    body: [
      "Order records are kept for as long as needed to support the transaction and meet accounting obligations. Account data is kept until you close your account, after which it is deleted or anonymised except where we must retain it.",
    ],
  },
  {
    heading: "6. Your choices",
    body: [
      "You can ask for a copy of your data, ask us to correct it, or ask us to delete it. Where deletion would break an order record we still owe the other party, we will explain what we can and cannot remove.",
      "Marketing email, if you have opted into it, always carries an unsubscribe link.",
    ],
  },
  {
    heading: "7. Contacting us",
    body: [
      "Privacy questions and requests go to hello@ownstall.app.",
    ],
  },
];
