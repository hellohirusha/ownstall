import { useState, useRef } from "react";
import { useNavigate } from "react-router-dom";
import { gql } from "@apollo/client";
import { useMutation } from "@apollo/client/react";
import { Upload, X, Loader2 } from "lucide-react";
import toast from "react-hot-toast";
import { Input } from "../../components/ui/Input";

const CREATE_PRODUCT = gql`
  mutation CreateProduct($input: CreateProductInput!) {
    createProduct(input: $input) {
      id
      name
      slug
    }
  }
`;

const ADD_PRODUCT_IMAGE = gql`
  mutation AddProductImage($productId: UUID!, $url: String!) {
    addProductImage(productId: $productId, url: $url) {
      id
      url
      position
    }
  }
`;

export function NewProductPage() {
  const navigate = useNavigate();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [uploadedImages, setUploadedImages] = useState<string[]>([]);
  const [uploadingImage, setUploadingImage] = useState(false);
  const [tags, setTags] = useState<string[]>([]);
  const [tagInput, setTagInput] = useState("");

  const [form, setForm] = useState({
    name: "",
    description: "",
    basePrice: "",
    comparePrice: "",
  });

  const [addProductImage] = useMutation<{
    addProductImage: { id: string; url: string; position: number };
  }>(ADD_PRODUCT_IMAGE);

  const [createProduct, { loading }] = useMutation<{
    createProduct: { id: string; name: string; slug: string };
  }>(CREATE_PRODUCT, {
    refetchQueries: ["GetProducts"],
    onCompleted: async (data) => {
      // Persist uploaded images in order so the first stays primary
      for (const url of uploadedImages) {
        try {
          await addProductImage({
            variables: { productId: data.createProduct.id, url },
          });
        } catch {
          toast.error("Failed to attach an image");
        }
      }
      toast.success("Product created!");
      navigate("/admin/products");
    },
    onError: (err) => {
      toast.error(err.message);
    },
  });

  const handleImageUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    setUploadingImage(true);
    const formData = new FormData();
    formData.append("image", file);

    try {
      const token = localStorage.getItem("access_token");
      const res = await fetch(
        `${process.env.REACT_APP_API_URL}/api/upload/product-image`,
        {
          method: "POST",
          headers: { Authorization: `Bearer ${token}` },
          body: formData,
        },
      );
      const data = await res.json();
      if (data.url) {
        setUploadedImages((prev) => [...prev, data.url]);
        toast.success("Image uploaded");
      } else {
        toast.error(data.error || "Image upload failed");
      }
    } catch {
      toast.error("Image upload failed");
    } finally {
      setUploadingImage(false);
    }
  };

  const handleAddTag = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && tagInput.trim()) {
      e.preventDefault();
      setTags((prev) => [...new Set([...prev, tagInput.trim().toLowerCase()])]);
      setTagInput("");
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!form.name || !form.basePrice) {
      toast.error("Name and price are required");
      return;
    }

    createProduct({
      variables: {
        input: {
          name: form.name,
          description: form.description || null,
          basePrice: parseFloat(form.basePrice),
          comparePrice: form.comparePrice
            ? parseFloat(form.comparePrice)
            : null,
          tags,
        },
      },
    });
  };

  return (
    <div className="p-6 max-w-2xl mx-auto">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-gray-900">New product</h1>
        <p className="text-gray-500 text-sm mt-1">Fill in the details below</p>
      </div>

      <form onSubmit={handleSubmit} className="space-y-6">
        {/* Basic Info */}
        <div className="bg-white rounded-xl border border-gray-100 p-5 space-y-4">
          <h2 className="font-semibold text-gray-900">Basic information</h2>
          <Input
            label="Product name *"
            value={form.name}
            onChange={(e) => setForm((p) => ({ ...p, name: e.target.value }))}
            placeholder="e.g. Custom Vinyl Sticker Sheet"
            required
          />
          <div>
            <label className="text-sm font-medium text-gray-700">
              Description
            </label>
            <textarea
              className="mt-1 w-full px-3 py-2 border border-gray-300 rounded-lg text-sm
                         focus:outline-none focus:ring-2 focus:ring-brand-500 resize-none"
              rows={4}
              value={form.description}
              onChange={(e) =>
                setForm((p) => ({ ...p, description: e.target.value }))
              }
              placeholder="Describe your product..."
            />
          </div>
        </div>

        {/* Pricing */}
        <div className="bg-white rounded-xl border border-gray-100 p-5 space-y-4">
          <h2 className="font-semibold text-gray-900">Pricing</h2>
          <div className="grid grid-cols-2 gap-4">
            <Input
              label="Price *"
              type="number"
              step="0.01"
              min="0"
              value={form.basePrice}
              onChange={(e) =>
                setForm((p) => ({ ...p, basePrice: e.target.value }))
              }
              placeholder="9.99"
              required
            />
            <Input
              label="Compare-at price"
              type="number"
              step="0.01"
              min="0"
              value={form.comparePrice}
              onChange={(e) =>
                setForm((p) => ({ ...p, comparePrice: e.target.value }))
              }
              placeholder="14.99"
            />
          </div>
          {form.comparePrice && form.basePrice && (
            <p className="text-xs text-green-600">
              {Math.round(
                ((parseFloat(form.comparePrice) - parseFloat(form.basePrice)) /
                  parseFloat(form.comparePrice)) *
                  100,
              )}
              % off — discount badge will show on storefront
            </p>
          )}
        </div>

        {/* Images */}
        <div className="bg-white rounded-xl border border-gray-100 p-5">
          <h2 className="font-semibold text-gray-900 mb-4">Images</h2>
          <div className="grid grid-cols-3 gap-3">
            {uploadedImages.map((url, i) => (
              <div
                key={i}
                className="relative aspect-square rounded-lg overflow-hidden border"
              >
                <img src={url} alt="" className="w-full h-full object-cover" />
                <button
                  type="button"
                  onClick={() =>
                    setUploadedImages((p) => p.filter((_, j) => j !== i))
                  }
                  className="absolute top-1 right-1 bg-black/50 text-white rounded-full p-0.5 hover:bg-black/70"
                >
                  <X size={12} />
                </button>
                {i === 0 && (
                  <div className="absolute bottom-1 left-1 bg-black/60 text-white text-xs px-1.5 py-0.5 rounded">
                    Primary
                  </div>
                )}
              </div>
            ))}

            {/* Upload button */}
            <button
              type="button"
              onClick={() => fileInputRef.current?.click()}
              disabled={uploadingImage}
              className="aspect-square rounded-lg border-2 border-dashed border-gray-200
                         flex flex-col items-center justify-center gap-1 hover:border-brand-400
                         hover:bg-green-50 transition-colors disabled:opacity-50"
            >
              {uploadingImage ? (
                <Loader2 className="h-6 w-6 text-gray-400 animate-spin" />
              ) : (
                <>
                  <Upload className="h-6 w-6 text-gray-400" />
                  <span className="text-xs text-gray-400">Upload</span>
                </>
              )}
            </button>
          </div>
          <input
            ref={fileInputRef}
            type="file"
            accept="image/*"
            className="hidden"
            onChange={handleImageUpload}
          />
          <p className="text-xs text-gray-400 mt-2">
            First image is the primary. Max 10MB each.
          </p>
        </div>

        {/* Tags */}
        <div className="bg-white rounded-xl border border-gray-100 p-5">
          <h2 className="font-semibold text-gray-900 mb-3">Tags</h2>
          <Input
            placeholder="Type a tag and press Enter"
            value={tagInput}
            onChange={(e) => setTagInput(e.target.value)}
            onKeyDown={handleAddTag}
          />
          <div className="flex flex-wrap gap-2 mt-3">
            {tags.map((tag) => (
              <span
                key={tag}
                className="flex items-center gap-1 px-2.5 py-1 bg-gray-100 rounded-full text-sm text-gray-700"
              >
                {tag}
                <button
                  type="button"
                  onClick={() => setTags((p) => p.filter((t) => t !== tag))}
                >
                  <X size={12} />
                </button>
              </span>
            ))}
          </div>
        </div>

        {/* Submit */}
        <div className="flex gap-3">
          <button
            type="button"
            onClick={() => navigate("/admin/products")}
            className="flex-1 py-2.5 border border-gray-200 rounded-lg text-gray-700
                       hover:bg-gray-50 transition-colors font-medium text-sm"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={loading}
            className="flex-1 py-2.5 bg-brand-600 hover:bg-brand-700 text-white font-medium
                       rounded-lg transition-colors disabled:opacity-50 text-sm"
          >
            {loading ? "Creating..." : "Create product"}
          </button>
        </div>
      </form>
    </div>
  );
}
