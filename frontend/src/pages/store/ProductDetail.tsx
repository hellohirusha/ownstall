import { useState, useEffect } from "react";
import { Link, useParams, useNavigate } from "react-router-dom";
import { gql } from "@apollo/client";
import { useQuery } from "@apollo/client/react";
import { ShoppingCart, Check, ChevronLeft } from "lucide-react";
import toast from "react-hot-toast";
import { useCart } from "../../lib/cart";

const GET_PRODUCT = gql`
  query GetProductBySlug($tenantId: UUID!, $slug: String!) {
    productBySlug(tenantId: $tenantId, slug: $slug) {
      id
      name
      description
      basePrice
      comparePrice
      tags
      images {
        url
        altText
        position
      }
      variants {
        id
        sku
        title
        option1Name
        option1Value
        option2Name
        option2Value
        price
        comparePrice
        stockQuantity
        isInStock
        imageUrl
      }
    }
  }
`;

const GET_TENANT = gql`
  query GetTenantBySubdomain($subdomain: String!) {
    tenant(subdomain: $subdomain) {
      id
    }
  }
`;

export function ProductDetailPage() {
  const { subdomain, slug } = useParams<{ subdomain: string; slug: string }>();
  const navigate = useNavigate();
  const { addItem, totalItems } = useCart();

  const [selectedVariantId, setSelectedVariantId] = useState<string>("");
  const [selectedImageIndex, setSelectedImageIndex] = useState(0);
  const [added, setAdded] = useState(false);

  // Resolve the store from the /store/:subdomain/... path segment
  const { data: tenantData, loading: tenantLoading } = useQuery<{
    tenant?: { id: string };
  }>(GET_TENANT, {
    variables: { subdomain },
    skip: !subdomain,
  });
  const tenantId = tenantData?.tenant?.id ?? "";

  const { data, loading: productLoading } = useQuery<{ productBySlug: any }>(
    GET_PRODUCT,
    {
      variables: { tenantId, slug },
      skip: !tenantId || !slug,
    },
  );
  const loading = tenantLoading || productLoading;

  // Auto-select first in-stock variant once the product loads
  useEffect(() => {
    const first = data?.productBySlug?.variants?.find((v: any) => v.isInStock);
    if (first) setSelectedVariantId(first.id);
  }, [data]);

  const product = data?.productBySlug;
  const selectedVariant = product?.variants?.find(
    (v: any) => v.id === selectedVariantId,
  );

  const handleAddToCart = () => {
    if (!product || !selectedVariant) return;

    addItem({
      variantId: selectedVariant.id,
      productId: product.id,
      productName: product.name,
      variantTitle: selectedVariant.title,
      price: selectedVariant.price,
      quantity: 1,
      imageUrl: product.images[0]?.url,
    });

    setAdded(true);
    toast.success(`${product.name} added to cart`);
    setTimeout(() => setAdded(false), 2000);
  };

  if (loading) {
    return (
      <div className="max-w-5xl mx-auto px-4 py-8">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
          <div className="aspect-square bg-gray-100 rounded-2xl animate-pulse" />
          <div className="space-y-4">
            <div className="h-8 bg-gray-100 rounded w-3/4 animate-pulse" />
            <div className="h-6 bg-gray-100 rounded w-1/4 animate-pulse" />
          </div>
        </div>
      </div>
    );
  }

  if (!product) {
    return (
      <div className="max-w-5xl mx-auto px-4 py-16 text-center">
        <p className="text-gray-500">Product not found</p>
      </div>
    );
  }

  // Group variants by option — builds the variant selector UI
  const option1Values = [
    ...new Set(
      product.variants
        .filter((v: any) => v.option1Name)
        .map((v: any) => v.option1Value),
    ),
  ];

  const displayPrice = selectedVariant?.price ?? product.basePrice;
  const displayCompare = selectedVariant?.comparePrice ?? product.comparePrice;

  return (
    <div className="max-w-5xl mx-auto px-4 py-8">
      {/* Back button + cart */}
      <div className="flex items-center justify-between mb-6">
        <button
          onClick={() => navigate(-1)}
          className="flex items-center gap-1 text-sm text-gray-500 hover:text-gray-900"
        >
          <ChevronLeft size={16} />
          Back
        </button>
        <Link
          to="/cart"
          className="flex items-center gap-2 px-4 py-2 bg-gray-900 text-white
                     rounded-lg text-sm font-medium hover:bg-gray-800 transition-colors"
        >
          <ShoppingCart size={16} />
          <span>Cart ({totalItems()})</span>
        </Link>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-10">
        {/* Images */}
        <div className="space-y-3">
          <div className="aspect-square bg-gray-50 rounded-2xl overflow-hidden">
            {product.images[selectedImageIndex] ? (
              <img
                src={product.images[selectedImageIndex].url}
                alt={product.name}
                className="w-full h-full object-cover"
              />
            ) : (
              <div className="w-full h-full bg-gradient-to-br from-gray-100 to-gray-200" />
            )}
          </div>
          {product.images.length > 1 && (
            <div className="flex gap-2">
              {product.images.map((img: any, i: number) => (
                <button
                  key={i}
                  onClick={() => setSelectedImageIndex(i)}
                  className={`w-16 h-16 rounded-lg overflow-hidden border-2 transition-colors
                    ${i === selectedImageIndex ? "border-brand-600" : "border-transparent"}`}
                >
                  <img
                    src={img.url}
                    alt=""
                    className="w-full h-full object-cover"
                  />
                </button>
              ))}
            </div>
          )}
        </div>

        {/* Product info */}
        <div>
          <h1 className="text-3xl font-bold text-gray-900 mb-2">
            {product.name}
          </h1>

          {/* Price */}
          <div className="flex items-baseline gap-3 mb-6">
            <span className="text-3xl font-bold text-gray-900">
              ${displayPrice.toFixed(2)}
            </span>
            {displayCompare && (
              <span className="text-xl text-gray-400 line-through">
                ${displayCompare.toFixed(2)}
              </span>
            )}
            {displayCompare && (
              <span className="px-2 py-0.5 bg-red-100 text-red-700 text-sm font-medium rounded-full">
                Save ${(displayCompare - displayPrice).toFixed(2)}
              </span>
            )}
          </div>

          {/* Variant selector */}
          {option1Values.length > 0 && (
            <div className="mb-6">
              <p className="text-sm font-medium text-gray-700 mb-2">
                {product.variants[0].option1Name}
              </p>
              <div className="flex flex-wrap gap-2">
                {option1Values.map((value: any) => {
                  const variant = product.variants.find(
                    (v: any) => v.option1Value === value,
                  );
                  const isSelected = selectedVariantId === variant?.id;
                  const inStock = variant?.isInStock;

                  return (
                    <button
                      key={value}
                      onClick={() =>
                        variant && setSelectedVariantId(variant.id)
                      }
                      disabled={!inStock}
                      className={`
                        px-4 py-2 rounded-lg border text-sm font-medium transition-all
                        ${
                          isSelected
                            ? "border-brand-600 bg-brand-50 text-brand-700"
                            : inStock
                              ? "border-gray-200 hover:border-gray-300 text-gray-700"
                              : "border-gray-100 text-gray-300 cursor-not-allowed line-through"
                        }
                      `}
                    >
                      {value}
                    </button>
                  );
                })}
              </div>
            </div>
          )}

          {/* Stock status */}
          {selectedVariant && (
            <p
              className={`text-sm mb-6 ${selectedVariant.isInStock ? "text-green-600" : "text-red-500"}`}
            >
              {selectedVariant.isInStock
                ? selectedVariant.stockQuantity <= 5
                  ? `Only ${selectedVariant.stockQuantity} left!`
                  : "In stock"
                : "Out of stock"}
            </p>
          )}

          {/* Add to cart button */}
          <button
            onClick={handleAddToCart}
            disabled={!selectedVariant?.isInStock || added}
            className={`
              w-full py-4 rounded-xl font-semibold text-lg transition-all
              flex items-center justify-center gap-2
              ${
                added
                  ? "bg-brand-600 text-white"
                  : selectedVariant?.isInStock
                    ? "bg-gray-900 hover:bg-gray-800 text-white"
                    : "bg-gray-100 text-gray-400 cursor-not-allowed"
              }
            `}
          >
            {added ? (
              <>
                <Check size={20} />
                Added to cart!
              </>
            ) : (
              <>
                <ShoppingCart size={20} />
                Add to cart — ${displayPrice.toFixed(2)}
              </>
            )}
          </button>

          {/* Description */}
          {product.description && (
            <div className="mt-8 pt-8 border-t border-gray-100">
              <h3 className="font-semibold text-gray-900 mb-3">Description</h3>
              <p className="text-gray-600 leading-relaxed whitespace-pre-line">
                {product.description}
              </p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
