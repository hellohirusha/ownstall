import { useState } from "react";
import { gql } from "@apollo/client";
import { useMutation, useQuery } from "@apollo/client/react";
import { useNavigate } from "react-router-dom";
import { Star, DollarSign, Edit3, Plus, Briefcase, CheckCircle } from "lucide-react";
import toast from "react-hot-toast";
import { AdminNav } from "../../components/AdminNav";
import { Input } from "../../components/ui/Input";

const GET_HIRE_ME_PROFILE = gql`
  query GetHireMeProfile {
    myCreatorProfile {
      id
      displayName
      tagline
      bio
      avatarUrl
      skills
      isAvailable
      hourlyRate
      isPublished
      stripeOnboarded
      stripeAccountId
      totalBookings
      completedBookings
      avgRating
      totalReviews
      services {
        id
        title
        price
        deliveryDays
        isActive
      }
      portfolioItems {
        id
        title
        imageUrl
        position
      }
      bookings(status: "pending") {
        id
        title
        clientName
        clientEmail
        agreedPrice
        status
        createdAt
      }
    }
  }
`;

const UPDATE_PROFILE = gql`
  mutation UpdateCreatorProfile($input: UpdateCreatorProfileInput!) {
    updateCreatorProfile(input: $input) {
      id
      displayName
      isPublished
    }
  }
`;

const CREATE_SERVICE = gql`
  mutation CreateCreatorService($input: CreateServiceInput!) {
    createCreatorService(input: $input) {
      id
      title
      price
    }
  }
`;

const CONNECT_STRIPE = gql`
  mutation ConnectStripe {
    generateStripeOnboardingLink {
      url
    }
  }
`;

const ACCEPT_BOOKING = gql`
  mutation AcceptBooking($id: UUID!) {
    acceptBooking(id: $id) {
      id
      status
    }
  }
`;

const DECLINE_BOOKING = gql`
  mutation DeclineBooking($id: UUID!) {
    declineBooking(id: $id) {
      id
      status
    }
  }
`;

export function HireMePage() {
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState<"overview" | "services" | "bookings" | "portfolio">("overview");
  const [editMode, setEditMode] = useState(false);
  const [showNewService, setShowNewService] = useState(false);

  const { data, loading, refetch } = useQuery<{ myCreatorProfile: any }>(GET_HIRE_ME_PROFILE);
  const [updateProfile] = useMutation(UPDATE_PROFILE, {
    onCompleted: () => {
      toast.success("Profile saved");
      setEditMode(false);
      refetch();
    },
    onError: (e) => toast.error(e.message),
  });
  const [connectStripe] = useMutation<{ generateStripeOnboardingLink: { url: string } }>(CONNECT_STRIPE, {
    onCompleted: (d) => {
      window.location.href = d.generateStripeOnboardingLink.url;
    },
    onError: (e) => toast.error(e.message),
  });
  const [createService, { loading: creatingService }] = useMutation(CREATE_SERVICE, {
    onCompleted: () => {
      toast.success("Service created");
      setShowNewService(false);
      setServiceForm({ title: "", description: "", price: "", deliveryDays: "7", revisions: "2" });
      refetch();
    },
    onError: (e) => toast.error(e.message),
  });
  const [acceptBooking] = useMutation(ACCEPT_BOOKING, {
    onCompleted: () => {
      toast.success("Booking accepted!");
      refetch();
    },
    onError: (e) => toast.error(e.message),
    refetchQueries: ["GetHireMeProfile"],
  });
  const [declineBooking, { loading: declining }] = useMutation(DECLINE_BOOKING, {
    onCompleted: () => {
      toast.success("Booking declined");
      refetch();
    },
    onError: (e) => toast.error(e.message),
    refetchQueries: ["GetHireMeProfile"],
  });

  const profile = data?.myCreatorProfile;
  const pendingBookings = profile?.bookings ?? [];

  const [profileForm, setProfileForm] = useState({
    displayName: "",
    tagline: "",
    bio: "",
    hourlyRate: "",
    skills: "",
    isAvailable: true,
  });

  const [serviceForm, setServiceForm] = useState({
    title: "",
    description: "",
    price: "",
    deliveryDays: "7",
    revisions: "2",
  });

  // Populate the form from the loaded profile when entering edit mode
  const startEditing = () => {
    setProfileForm({
      displayName: profile?.displayName ?? "",
      tagline: profile?.tagline ?? "",
      bio: profile?.bio ?? "",
      hourlyRate: profile?.hourlyRate?.toString() ?? "",
      skills: profile?.skills?.join(", ") ?? "",
      isAvailable: profile?.isAvailable ?? true,
    });
    setEditMode(true);
  };

  const handleCreateService = () => {
    if (!serviceForm.title.trim() || !serviceForm.price) return;
    createService({
      variables: {
        input: {
          title: serviceForm.title,
          description: serviceForm.description || null,
          price: parseFloat(serviceForm.price),
          deliveryDays: parseInt(serviceForm.deliveryDays, 10) || 7,
          revisions: parseInt(serviceForm.revisions, 10) || 2,
        },
      },
    });
  };

  if (loading)
    return (
      <div className="min-h-screen bg-gray-50">
        <AdminNav />
        <div className="p-6 text-center text-gray-400">Loading...</div>
      </div>
    );

  return (
    <div className="min-h-screen bg-gray-50">
      <AdminNav />
      <div className="p-6 max-w-4xl mx-auto">
        {/* Header */}
        <div className="flex items-center justify-between mb-6">
          <div>
            <h1 className="text-2xl font-bold text-gray-900">Hire Me</h1>
            <p className="text-sm text-gray-500 mt-1">
              {profile?.isPublished
                ? "Your profile is live"
                : "Your profile is not published yet"}
            </p>
          </div>
          <div className="flex items-center gap-2">
            {/* Stripe Connect button */}
            {profile && !profile.stripeOnboarded && (
              <button
                onClick={() => connectStripe()}
                className="flex items-center gap-2 px-4 py-2 bg-purple-600 hover:bg-purple-700
                           text-white text-sm font-medium rounded-lg transition-colors"
              >
                <DollarSign size={14} />
                Connect Stripe to receive payments
              </button>
            )}
            {profile && !profile.isPublished && (
              <button
                onClick={() => updateProfile({ variables: { input: { isPublished: true } } })}
                className="px-4 py-2 bg-green-500 text-white text-sm font-medium rounded-lg"
              >
                Publish profile
              </button>
            )}
          </div>
        </div>

        {/* Stats */}
        <div className="grid grid-cols-4 gap-4 mb-6">
          {[
            { label: "Bookings", value: profile?.totalBookings ?? 0, icon: Briefcase },
            { label: "Completed", value: profile?.completedBookings ?? 0, icon: CheckCircle },
            { label: "Reviews", value: profile?.totalReviews ?? 0, icon: Star },
            { label: "Avg rating", value: profile?.avgRating ? `${profile.avgRating}★` : "—", icon: Star },
          ].map(({ label, value }) => (
            <div key={label} className="bg-white rounded-xl border border-gray-100 p-4">
              <p className="text-2xl font-bold text-gray-900">{value}</p>
              <p className="text-xs text-gray-500 mt-0.5">{label}</p>
            </div>
          ))}
        </div>

        {/* Tabs */}
        <div className="flex gap-1 bg-gray-100 p-1 rounded-xl mb-6 w-fit">
          {(["overview", "services", "bookings", "portfolio"] as const).map((tab) => (
            <button
              key={tab}
              onClick={() => setActiveTab(tab)}
              className={`px-4 py-1.5 rounded-lg text-sm font-medium capitalize transition-colors
                ${
                  activeTab === tab
                    ? "bg-white text-gray-900 shadow-sm"
                    : "text-gray-500 hover:text-gray-700"
                }`}
            >
              {tab}
              {tab === "bookings" && pendingBookings.length > 0 && (
                <span className="ml-1.5 bg-red-500 text-white text-xs rounded-full px-1.5">
                  {pendingBookings.length}
                </span>
              )}
            </button>
          ))}
        </div>

        {/* ── Overview ─────────────────────────────────── */}
        {activeTab === "overview" && (
          <div className="bg-white rounded-xl border border-gray-100 p-6">
            <div className="flex items-center justify-between mb-4">
              <h2 className="font-semibold text-gray-900">Profile details</h2>
              <button
                onClick={() => (editMode ? setEditMode(false) : startEditing())}
                className="flex items-center gap-1 text-sm text-gray-500 hover:text-gray-900"
              >
                <Edit3 size={14} />
                {editMode ? "Cancel" : "Edit"}
              </button>
            </div>

            {editMode ? (
              <div className="space-y-4">
                <Input
                  label="Display name"
                  value={profileForm.displayName}
                  onChange={(e) => setProfileForm((p) => ({ ...p, displayName: e.target.value }))}
                />
                <Input
                  label="Tagline"
                  value={profileForm.tagline}
                  placeholder="Full-stack developer specializing in Go and React"
                  onChange={(e) => setProfileForm((p) => ({ ...p, tagline: e.target.value }))}
                />
                <div>
                  <label className="text-sm font-medium text-gray-700 block mb-1">Bio</label>
                  <textarea
                    value={profileForm.bio}
                    onChange={(e) => setProfileForm((p) => ({ ...p, bio: e.target.value }))}
                    rows={4}
                    className="w-full px-3 py-2 border border-gray-200 rounded-lg text-sm
                               focus:outline-none focus:ring-2 focus:ring-green-500 resize-none"
                  />
                </div>
                <Input
                  label="Hourly rate (USD)"
                  type="number"
                  value={profileForm.hourlyRate}
                  placeholder="150"
                  onChange={(e) => setProfileForm((p) => ({ ...p, hourlyRate: e.target.value }))}
                />
                <Input
                  label="Skills (comma separated)"
                  value={profileForm.skills}
                  placeholder="Go, React, GraphQL, TypeScript"
                  onChange={(e) => setProfileForm((p) => ({ ...p, skills: e.target.value }))}
                />
                <div className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    id="available"
                    checked={profileForm.isAvailable}
                    onChange={(e) => setProfileForm((p) => ({ ...p, isAvailable: e.target.checked }))}
                  />
                  <label htmlFor="available" className="text-sm text-gray-700">
                    Available for new work
                  </label>
                </div>
                <button
                  onClick={() =>
                    updateProfile({
                      variables: {
                        input: {
                          displayName: profileForm.displayName || null,
                          tagline: profileForm.tagline || null,
                          bio: profileForm.bio || null,
                          hourlyRate: profileForm.hourlyRate ? parseFloat(profileForm.hourlyRate) : null,
                          skills: profileForm.skills
                            .split(",")
                            .map((s) => s.trim())
                            .filter(Boolean),
                          isAvailable: profileForm.isAvailable,
                        },
                      },
                    })
                  }
                  className="w-full py-2.5 bg-green-500 text-white rounded-lg font-medium text-sm"
                >
                  Save changes
                </button>
              </div>
            ) : (
              <div className="space-y-3">
                <div className="flex items-center gap-3">
                  <div
                    className="w-16 h-16 bg-gradient-to-br from-green-400 to-blue-500
                               rounded-full flex items-center justify-center text-2xl font-bold text-white"
                  >
                    {(profile?.displayName ?? "?")[0]}
                  </div>
                  <div>
                    <p className="font-semibold text-gray-900">
                      {profile?.displayName ?? "No profile yet"}
                    </p>
                    <p className="text-sm text-gray-500">
                      {profile?.tagline ?? "Click Edit to create your creator profile"}
                    </p>
                  </div>
                </div>
                {profile?.bio && <p className="text-sm text-gray-600">{profile.bio}</p>}
                <div className="flex flex-wrap gap-2">
                  {profile?.skills?.map((s: string) => (
                    <span key={s} className="px-2.5 py-1 bg-gray-100 rounded-full text-xs text-gray-700">
                      {s}
                    </span>
                  ))}
                </div>
                {profile?.hourlyRate && (
                  <p className="text-sm text-gray-600">
                    <strong>${profile.hourlyRate}/hr</strong>
                  </p>
                )}
              </div>
            )}
          </div>
        )}

        {/* ── Services ─────────────────────────────────── */}
        {activeTab === "services" && (
          <div className="space-y-4">
            {profile?.services?.map((svc: any) => (
              <div
                key={svc.id}
                className="bg-white rounded-xl border border-gray-100 p-4
                           flex items-center justify-between"
              >
                <div>
                  <p className="font-medium text-gray-900">{svc.title}</p>
                  <p className="text-sm text-gray-500">
                    ${svc.price} · {svc.deliveryDays} day delivery
                  </p>
                </div>
                <span
                  className={`text-xs px-2 py-0.5 rounded-full ${
                    svc.isActive ? "bg-green-100 text-green-700" : "bg-gray-100 text-gray-500"
                  }`}
                >
                  {svc.isActive ? "Active" : "Inactive"}
                </span>
              </div>
            ))}

            {showNewService ? (
              <div className="bg-white rounded-xl border border-gray-100 p-5 space-y-4">
                <h3 className="font-semibold text-gray-900">New service</h3>
                <Input
                  label="Title"
                  value={serviceForm.title}
                  onChange={(e) => setServiceForm((p) => ({ ...p, title: e.target.value }))}
                  placeholder="Brand Identity Package"
                />
                <div className="grid grid-cols-3 gap-3">
                  <Input
                    label="Price (USD)"
                    type="number"
                    value={serviceForm.price}
                    onChange={(e) => setServiceForm((p) => ({ ...p, price: e.target.value }))}
                  />
                  <Input
                    label="Delivery (days)"
                    type="number"
                    value={serviceForm.deliveryDays}
                    onChange={(e) => setServiceForm((p) => ({ ...p, deliveryDays: e.target.value }))}
                  />
                  <Input
                    label="Revisions"
                    type="number"
                    value={serviceForm.revisions}
                    onChange={(e) => setServiceForm((p) => ({ ...p, revisions: e.target.value }))}
                  />
                </div>
                <div className="flex gap-2">
                  <button
                    onClick={() => setShowNewService(false)}
                    className="flex-1 py-2 border border-gray-200 rounded-lg text-sm text-gray-600"
                  >
                    Cancel
                  </button>
                  <button
                    onClick={handleCreateService}
                    disabled={creatingService || !serviceForm.title.trim() || !serviceForm.price}
                    className="flex-1 py-2 bg-green-500 text-white rounded-lg text-sm font-medium
                               disabled:opacity-50"
                  >
                    {creatingService ? "Creating..." : "Create service"}
                  </button>
                </div>
              </div>
            ) : (
              <button
                onClick={() => setShowNewService(true)}
                className="w-full py-3 border-2 border-dashed border-gray-200 rounded-xl
                           flex items-center justify-center gap-2 text-sm text-gray-400
                           hover:border-green-400 hover:text-green-500 transition-colors"
              >
                <Plus size={16} />
                Add service package
              </button>
            )}
          </div>
        )}

        {/* ── Bookings ─────────────────────────────────── */}
        {activeTab === "bookings" && (
          <div className="space-y-4">
            {pendingBookings.length === 0 ? (
              <div className="text-center py-12 text-gray-400">
                <Briefcase className="mx-auto h-12 w-12 text-gray-200 mb-3" />
                <p>No pending bookings</p>
              </div>
            ) : (
              pendingBookings.map((b: any) => (
                <div key={b.id} className="bg-white rounded-xl border border-gray-100 p-4">
                  <div className="flex justify-between items-start mb-2">
                    <button
                      onClick={() => navigate(`/admin/hire/bookings/${b.id}`)}
                      className="text-left"
                    >
                      <p className="font-medium text-gray-900 hover:underline">{b.title}</p>
                      <p className="text-sm text-gray-500">
                        {b.clientName} · {b.clientEmail}
                      </p>
                    </button>
                    <p className="font-bold text-green-600">${b.agreedPrice}</p>
                  </div>
                  <div className="flex gap-2 mt-3">
                    <button
                      onClick={() => acceptBooking({ variables: { id: b.id } })}
                      className="flex-1 py-2 bg-green-500 text-white text-sm rounded-lg font-medium"
                    >
                      Accept
                    </button>
                    <button
                      onClick={() => declineBooking({ variables: { id: b.id } })}
                      disabled={declining}
                      className="flex-1 py-2 border border-gray-200 text-gray-600 text-sm rounded-lg
                                 hover:bg-gray-50 transition-colors disabled:opacity-50"
                    >
                      Decline
                    </button>
                  </div>
                </div>
              ))
            )}
          </div>
        )}

        {/* ── Portfolio ────────────────────────────────── */}
        {activeTab === "portfolio" && (
          <div className="text-center py-12 text-gray-400">
            <p>
              {profile?.portfolioItems?.length
                ? `${profile.portfolioItems.length} portfolio item(s)`
                : "No portfolio items yet"}
            </p>
          </div>
        )}
      </div>
    </div>
  );
}
