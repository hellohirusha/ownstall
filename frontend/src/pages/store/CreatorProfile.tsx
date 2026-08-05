import { useState } from "react";
import { gql } from "@apollo/client";
import { useMutation, useQuery } from "@apollo/client/react";
import { Star, Clock, CheckCircle, DollarSign, Send } from "lucide-react";
import toast from "react-hot-toast";
import { Input } from "../../components/ui/Input";

const GET_PUBLIC_PROFILE = gql`
  query GetPublicProfile($tenantId: UUID!, $userId: UUID) {
    creatorProfile(tenantId: $tenantId, userId: $userId) {
      id
      displayName
      tagline
      bio
      avatarUrl
      skills
      isAvailable
      hourlyRate
      responseTime
      avgRating
      totalReviews
      completedBookings
      services {
        id
        title
        description
        price
        deliveryDays
        revisions
      }
      portfolioItems {
        id
        title
        imageUrl
        tags
      }
      reviews {
        id
        reviewerName
        rating
        body
        createdAt
      }
    }
  }
`;

const CREATE_BOOKING = gql`
  mutation CreateBooking($input: CreateBookingInput!) {
    createBooking(input: $input) {
      bookingId
      clientSecret
      agreedPrice
    }
  }
`;

export function CreatorProfilePage() {
  const tenantId = new URLSearchParams(window.location.search).get("tenantId") ?? "";
  const [selectedService, setSelectedService] = useState<any>(null);
  const [showBookingForm, setShowBookingForm] = useState(false);
  const [bookingForm, setBookingForm] = useState({
    clientEmail: "",
    clientName: "",
    title: "",
    description: "",
  });
  const [bookingSuccess, setBookingSuccess] = useState(false);

  const { data, loading } = useQuery<{ creatorProfile: any }>(GET_PUBLIC_PROFILE, {
    variables: { tenantId },
    skip: !tenantId,
  });

  const [createBooking, { loading: booking }] = useMutation(CREATE_BOOKING, {
    onCompleted: () => {
      setBookingSuccess(true);
      toast.success("Booking request sent!");
    },
    onError: (e) => toast.error(e.message),
  });

  const profile = data?.creatorProfile;

  if (loading)
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="w-8 h-8 border-2 border-green-500 border-t-transparent rounded-full animate-spin" />
      </div>
    );

  if (!profile)
    return (
      <div className="min-h-screen flex items-center justify-center">
        <p className="text-gray-500">Creator profile not found</p>
      </div>
    );

  const handleBook = () => {
    if (!selectedService) return;
    createBooking({
      variables: {
        input: {
          profileId: profile.id,
          serviceId: selectedService.id,
          ...bookingForm,
          agreedPrice: selectedService.price,
        },
      },
    });
  };

  return (
    <div className="min-h-screen bg-gray-50">
      <div className="max-w-4xl mx-auto px-4 py-10">
        {/* Profile header */}
        <div className="bg-white rounded-2xl p-8 mb-6 shadow-sm">
          <div className="flex items-start gap-6">
            <div
              className="w-20 h-20 rounded-2xl bg-gradient-to-br from-green-400 to-blue-500
                         flex items-center justify-center text-3xl font-bold text-white flex-shrink-0"
            >
              {profile.displayName[0]}
            </div>
            <div className="flex-1">
              <div className="flex items-center gap-3 mb-1">
                <h1 className="text-2xl font-bold text-gray-900">{profile.displayName}</h1>
                {profile.isAvailable && (
                  <span
                    className="flex items-center gap-1 text-xs text-green-600
                               bg-green-50 px-2.5 py-1 rounded-full font-medium"
                  >
                    <span className="w-1.5 h-1.5 bg-green-500 rounded-full animate-pulse" />
                    Available
                  </span>
                )}
              </div>
              <p className="text-gray-600 mb-3">{profile.tagline}</p>
              <div className="flex items-center gap-5 text-sm text-gray-500">
                {profile.avgRating && (
                  <span className="flex items-center gap-1">
                    <Star size={14} className="text-yellow-400 fill-yellow-400" />
                    {profile.avgRating} ({profile.totalReviews} reviews)
                  </span>
                )}
                <span className="flex items-center gap-1">
                  <CheckCircle size={14} className="text-green-500" />
                  {profile.completedBookings} projects completed
                </span>
                {profile.hourlyRate && (
                  <span className="flex items-center gap-1">
                    <DollarSign size={14} />${profile.hourlyRate}/hr
                  </span>
                )}
                <span className="flex items-center gap-1">
                  <Clock size={14} />
                  Responds {profile.responseTime?.replace(/_/g, " ")}
                </span>
              </div>
            </div>
          </div>

          {profile.bio && (
            <p className="text-gray-600 text-sm mt-4 leading-relaxed">{profile.bio}</p>
          )}

          <div className="flex flex-wrap gap-2 mt-4">
            {profile.skills?.map((skill: string) => (
              <span key={skill} className="px-3 py-1 bg-gray-100 rounded-full text-sm text-gray-700">
                {skill}
              </span>
            ))}
          </div>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
          {/* Services */}
          <div className="md:col-span-2 space-y-4">
            <h2 className="text-lg font-bold text-gray-900">Services</h2>
            {profile.services?.map((svc: any) => (
              <div
                key={svc.id}
                onClick={() => {
                  setSelectedService(svc);
                  setShowBookingForm(true);
                }}
                className={`bg-white rounded-xl p-5 border-2 cursor-pointer transition-all
                  ${
                    selectedService?.id === svc.id
                      ? "border-green-500 shadow-sm"
                      : "border-gray-100 hover:border-gray-200"
                  }`}
              >
                <div className="flex justify-between items-start">
                  <div className="flex-1">
                    <h3 className="font-semibold text-gray-900">{svc.title}</h3>
                    {svc.description && (
                      <p className="text-sm text-gray-500 mt-1">{svc.description}</p>
                    )}
                    <div className="flex gap-4 mt-2 text-xs text-gray-400">
                      <span>{svc.deliveryDays} day delivery</span>
                      <span>{svc.revisions} revisions</span>
                    </div>
                  </div>
                  <div className="text-right ml-4">
                    <p className="text-xl font-bold text-gray-900">${svc.price}</p>
                    <button
                      className="mt-2 px-4 py-1.5 bg-green-500 text-white text-xs
                                 rounded-lg font-medium hover:bg-green-600"
                    >
                      Book
                    </button>
                  </div>
                </div>
              </div>
            ))}

            {/* Portfolio */}
            {profile.portfolioItems?.length > 0 && (
              <>
                <h2 className="text-lg font-bold text-gray-900 mt-8">Portfolio</h2>
                <div className="grid grid-cols-2 gap-3">
                  {profile.portfolioItems.map((item: any) => (
                    <div key={item.id} className="bg-white rounded-xl overflow-hidden border border-gray-100">
                      <img src={item.imageUrl} alt={item.title} className="w-full h-40 object-cover" />
                      <div className="p-3">
                        <p className="text-sm font-medium text-gray-900">{item.title}</p>
                      </div>
                    </div>
                  ))}
                </div>
              </>
            )}
          </div>

          {/* Booking form */}
          <div className="space-y-4">
            {showBookingForm && selectedService && !bookingSuccess ? (
              <div className="bg-white rounded-xl border border-gray-100 p-5 sticky top-4">
                <h3 className="font-semibold text-gray-900 mb-1">{selectedService.title}</h3>
                <p className="text-2xl font-bold text-green-600 mb-4">${selectedService.price}</p>
                <div className="space-y-3">
                  <Input
                    label="Your name"
                    value={bookingForm.clientName}
                    onChange={(e) => setBookingForm((p) => ({ ...p, clientName: e.target.value }))}
                  />
                  <Input
                    label="Your email"
                    type="email"
                    value={bookingForm.clientEmail}
                    onChange={(e) => setBookingForm((p) => ({ ...p, clientEmail: e.target.value }))}
                  />
                  <Input
                    label="Project title"
                    value={bookingForm.title}
                    placeholder="Logo redesign for my startup"
                    onChange={(e) => setBookingForm((p) => ({ ...p, title: e.target.value }))}
                  />
                  <div>
                    <label className="text-sm font-medium text-gray-700 block mb-1">Brief</label>
                    <textarea
                      value={bookingForm.description}
                      onChange={(e) => setBookingForm((p) => ({ ...p, description: e.target.value }))}
                      rows={3}
                      placeholder="Describe your project..."
                      className="w-full px-3 py-2 border border-gray-200 rounded-lg text-sm
                                 focus:outline-none focus:ring-2 focus:ring-green-500 resize-none"
                    />
                  </div>
                  <button
                    onClick={handleBook}
                    disabled={booking || !bookingForm.clientEmail || !bookingForm.description || !bookingForm.title}
                    className="w-full py-3 bg-green-500 hover:bg-green-600 text-white
                               font-semibold rounded-xl transition-colors disabled:opacity-50
                               flex items-center justify-center gap-2"
                  >
                    <Send size={16} />
                    {booking ? "Sending..." : "Send booking request"}
                  </button>
                  <p className="text-xs text-gray-400 text-center">
                    10% platform fee applies. Funds held until delivery.
                  </p>
                </div>
              </div>
            ) : bookingSuccess ? (
              <div className="bg-green-50 border border-green-200 rounded-xl p-5 text-center">
                <CheckCircle className="mx-auto h-10 w-10 text-green-500 mb-2" />
                <p className="font-semibold text-green-800">Booking request sent!</p>
                <p className="text-sm text-green-600 mt-1">
                  {profile.displayName} will respond {profile.responseTime?.replace(/_/g, " ")}.
                </p>
              </div>
            ) : (
              <div className="bg-white rounded-xl border border-gray-100 p-5 text-center">
                <p className="text-sm text-gray-400">Select a service to get started</p>
              </div>
            )}

            {/* Reviews */}
            {profile.reviews?.length > 0 && (
              <div className="bg-white rounded-xl border border-gray-100 p-5">
                <h3 className="font-semibold text-gray-900 mb-3">Reviews</h3>
                <div className="space-y-3">
                  {profile.reviews.slice(0, 3).map((r: any) => (
                    <div key={r.id} className="border-b border-gray-50 pb-3 last:border-0">
                      <div className="flex items-center justify-between mb-1">
                        <p className="text-sm font-medium text-gray-900">{r.reviewerName}</p>
                        <div className="flex">
                          {[...Array(5)].map((_, i) => (
                            <Star
                              key={i}
                              size={12}
                              className={i < r.rating ? "text-yellow-400 fill-yellow-400" : "text-gray-200"}
                            />
                          ))}
                        </div>
                      </div>
                      {r.body && <p className="text-xs text-gray-500">{r.body}</p>}
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
