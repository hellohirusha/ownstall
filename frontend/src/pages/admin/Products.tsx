import { gql } from "@apollo/client";
import { useMutation, useQuery } from "@apollo/client/react";
import { Link } from "react-router-dom";
import { Plus, Package, AlertCircle } from "lucide-react";
import { AdminNav } from "../../components/AdminNav";

const GET_PRODUCTS = gql`
  query GetProducts {
    products {
      id
      name
      slug
      basePrice
      status
      isFeatured
      tags
      images {
        url
        position
      }
      variants {
        id
        stockQuantity
        isInStock
      }
      createdAt
    }
  }
`;

const PUBLISH_PRODUCT = gql`
  mutation PublishProduct($id: UUID!) {
    publishProduct(id: $id) {
      id
      status
    }
  }
`;

export function ProductsPage() {
  const { data, loading, error } = useQuery<{ products: any[] }>(GET_PRODUCTS);
  const [publishProduct, { loading: publishing }] = useMutation<{
    publishProduct: { id: string; status: string };
  }>(PUBLISH_PRODUCT, { refetchQueries: ["GetProducts"] });

  if (loading) {
    return (
      <>
        <AdminNav />
        <div className="p-6">
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {[...Array(6)].map((_, i) => (
              <div
                key={i}
                className="h-48 bg-gray-100 rounded-xl animate-pulse"
              />
            ))}
          </div>
        </div>
      </>
    );
  }

  if (error) {
    return (
      <>
        <AdminNav />
        <div className="p-6 text-center">
          <AlertCircle className="mx-auto h-12 w-12 text-red-400 mb-3" />
          <p className="text-gray-600">Failed to load products</p>
        </div>
      </>
    );
  }

  const products = data?.products ?? [];

  return (
    <>
      <AdminNav />
      <div className="p-6">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Products</h1>
          <p className="text-sm text-gray-500 mt-1">
            {products.length} {products.length === 1 ? "product" : "products"}
          </p>
        </div>
        <Link
          to="/admin/products/new"
          className="flex items-center gap-2 px-4 py-2 bg-brand-600 hover:bg-brand-700
                     text-white font-medium rounded-lg transition-colors text-sm"
        >
          <Plus size={16} />
          Add product
        </Link>
      </div>

      {/* Empty state */}
      {products.length === 0 && (
        <div className="text-center py-16">
          <Package className="mx-auto h-16 w-16 text-gray-300 mb-4" />
          <h3 className="text-lg font-medium text-gray-900 mb-2">
            No products yet
          </h3>
          <p className="text-gray-500 mb-6">
            Start by adding your first product.
          </p>
          <Link
            to="/admin/products/new"
            className="px-5 py-2.5 bg-brand-600 text-white rounded-lg font-medium
                       hover:bg-brand-700 transition-colors"
          >
            Add your first product
          </Link>
        </div>
      )}

      {/* Product grid */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        {products.map((product: any) => {
          const primaryImage = product.images.find(
            (img: any) => img.position === 0,
          );
          const totalStock = product.variants.reduce(
            (sum: number, v: any) => sum + v.stockQuantity,
            0,
          );
          const inStock = product.variants.some((v: any) => v.isInStock);

          return (
            <Link
              key={product.id}
              to={`/admin/products/${product.id}`}
              className="group bg-white rounded-xl border border-gray-100 overflow-hidden
                         hover:shadow-md transition-shadow"
            >
              {/* Product image */}
              <div className="aspect-square bg-gray-50 relative overflow-hidden">
                {primaryImage ? (
                  <img
                    src={primaryImage.url}
                    alt={product.name}
                    className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-300"
                  />
                ) : (
                  <div className="w-full h-full flex items-center justify-center">
                    <Package className="h-12 w-12 text-gray-300" />
                  </div>
                )}
                {/* Status badge */}
                <div
                  className={`
                  absolute top-2 right-2 px-2 py-0.5 rounded-full text-xs font-medium
                  ${
                    product.status === "active"
                      ? "bg-green-100 text-green-700"
                      : product.status === "draft"
                        ? "bg-yellow-100 text-yellow-700"
                        : "bg-gray-100 text-gray-600"
                  }
                `}
                >
                  {product.status}
                </div>
              </div>

              {/* Product info */}
              <div className="p-3">
                <h3 className="font-medium text-gray-900 truncate">
                  {product.name}
                </h3>
                <div className="flex items-center justify-between mt-1">
                  <span className="text-green-600 font-semibold text-sm">
                    ${product.basePrice.toFixed(2)}
                  </span>
                  <span
                    className={`text-xs ${inStock ? "text-gray-500" : "text-red-500"}`}
                  >
                    {inStock ? `${totalStock} in stock` : "Out of stock"}
                  </span>
                </div>
                {product.status === "draft" && (
                  <button
                    type="button"
                    disabled={publishing}
                    onClick={(e) => {
                      e.preventDefault();
                      publishProduct({ variables: { id: product.id } });
                    }}
                    className="mt-2 w-full py-1.5 text-xs font-medium bg-gray-900 text-white
                               rounded-lg hover:bg-gray-800 transition-colors disabled:opacity-50"
                  >
                    {publishing ? "Publishing..." : "Publish to store"}
                  </button>
                )}
              </div>
            </Link>
          );
        })}
      </div>
      </div>
    </>
  );
}
