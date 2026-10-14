package auth

import (
	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
)

const (
	// public procedures need no session, signedIn ones need any account, and
	// worker ones need the worker token. unverified ones also allow an account
	// whose email isn't verified yet.
	public     krillv1.Permission = -1
	worker     krillv1.Permission = -2
	unverified krillv1.Permission = -3
	signedIn                      = krillv1.Permission_PERMISSION_UNSPECIFIED

	label            = krillv1.Permission_PERMISSION_LABEL
	manageVideos     = krillv1.Permission_PERMISSION_MANAGE_VIDEOS
	manageLabelTypes = krillv1.Permission_PERMISSION_MANAGE_LABEL_TYPES
	manageExports    = krillv1.Permission_PERMISSION_MANAGE_EXPORTS
	manageUsers      = krillv1.Permission_PERMISSION_MANAGE_USERS
	reviewLabels     = krillv1.Permission_PERMISSION_REVIEW_LABELS
)

// policy is the permission each procedure needs. Procedures missing from it
// are denied, and a test checks that every procedure is listed.
var policy = map[string]krillv1.Permission{
	krillv1connect.HealthServiceCheckProcedure: public,

	krillv1connect.AuthServiceGetSessionProcedure:     public,
	krillv1connect.AuthServiceLoginProcedure:          public,
	krillv1connect.AuthServiceRegisterProcedure:       public,
	krillv1connect.AuthServiceLogoutProcedure:         public,
	krillv1connect.AuthServiceChangePasswordProcedure: signedIn,
	// Fixing a mistyped email has to work before it is verified.
	krillv1connect.AuthServiceUpdateProfileProcedure:        unverified,
	krillv1connect.AuthServiceSendVerificationCodeProcedure: unverified,
	krillv1connect.AuthServiceVerifyEmailProcedure:          unverified,
	krillv1connect.AuthServiceRequestPasswordResetProcedure: public,
	krillv1connect.AuthServiceResetPasswordProcedure:        public,
	krillv1connect.AuthServiceCheckResetCodeProcedure:       public,
	krillv1connect.AuthServiceRequestSignInCodeProcedure:    public,
	krillv1connect.AuthServiceSignInWithCodeProcedure:       public,

	krillv1connect.VideoServiceListVideosProcedure:  label,
	krillv1connect.VideoServiceGetVideoProcedure:    label,
	krillv1connect.VideoServiceCreateVideoProcedure: manageVideos,
	krillv1connect.VideoServiceStartIngestProcedure: manageVideos,
	krillv1connect.VideoServiceUpdateVideoProcedure: manageVideos,
	krillv1connect.VideoServiceDeleteVideoProcedure: manageVideos,

	krillv1connect.ClipServiceGetClipProcedure: label,

	krillv1connect.QueueServiceGetQueueProcedure:      label,
	krillv1connect.QueueServiceClaimNextClipProcedure: label,

	krillv1connect.LabelServiceListLabelTypesProcedure:           label,
	krillv1connect.LabelServiceCreateLabelTypeProcedure:          manageLabelTypes,
	krillv1connect.LabelServiceUpdateLabelTypeProcedure:          manageLabelTypes,
	krillv1connect.LabelServiceDeleteLabelTypeProcedure:          manageLabelTypes,
	krillv1connect.LabelServiceReorderLabelTypesProcedure:        manageLabelTypes,
	krillv1connect.LabelServiceCreateLabelExampleUploadProcedure: manageLabelTypes,
	krillv1connect.LabelServiceAddLabelExampleProcedure:          manageLabelTypes,
	krillv1connect.LabelServiceDeleteLabelExampleProcedure:       manageLabelTypes,

	krillv1connect.AnnotationServiceCreateTrackProcedure:       label,
	krillv1connect.AnnotationServiceUpdateTrackProcedure:       label,
	krillv1connect.AnnotationServiceDeleteTrackProcedure:       label,
	krillv1connect.AnnotationServiceSetBoxProcedure:            label,
	krillv1connect.AnnotationServiceDeleteBoxProcedure:         label,
	krillv1connect.AnnotationServiceCopyBoxesProcedure:         label,
	krillv1connect.AnnotationServiceSetFrameStatusProcedure:    label,
	krillv1connect.AnnotationServiceTrackObjectProcedure:       label,
	krillv1connect.AnnotationServiceListTracksByTypeProcedure:  label,
	krillv1connect.AnnotationServiceSearchExemplarProcedure:    label,
	krillv1connect.AnnotationServiceGetExemplarSearchProcedure: label,

	krillv1connect.WorkerServiceNextTaskProcedure:        worker,
	krillv1connect.WorkerServiceReportTrackProcedure:     worker,
	krillv1connect.WorkerServiceReportExemplarsProcedure: worker,
	krillv1connect.WorkerServiceReportEmbedProcedure:     worker,
	krillv1connect.WorkerServiceFailTaskProcedure:        worker,

	krillv1connect.GoldServiceSetGoldFrameProcedure:    reviewLabels,
	krillv1connect.GoldServiceGetGoldCheckProcedure:    label,
	krillv1connect.GoldServiceSubmitGoldCheckProcedure: label,
	krillv1connect.GoldServiceGetGoldStatsProcedure:    reviewLabels,
	// Labelers may see their own attempts. The handlers check the rest.
	krillv1connect.GoldServiceGetGoldAttemptProcedure:   label,
	krillv1connect.GoldServiceListGoldAttemptsProcedure: label,

	krillv1connect.ReviewServiceFlagFrameProcedure:     label,
	krillv1connect.ReviewServiceListReviewsProcedure:   reviewLabels,
	krillv1connect.ReviewServiceCountReviewsProcedure:  reviewLabels,
	krillv1connect.ReviewServiceResolveReviewProcedure: reviewLabels,

	krillv1connect.StatsServiceGetLeaderboardProcedure: label,
	krillv1connect.StatsServiceGetProfileProcedure:     label,
	krillv1connect.StatsServiceGetDriftStatsProcedure:  label,

	krillv1connect.ExportServicePreviewExportProcedure: manageExports,
	krillv1connect.ExportServiceCreateExportProcedure:  manageExports,
	krillv1connect.ExportServiceListExportsProcedure:   manageExports,
	krillv1connect.ExportServiceDeleteExportProcedure:  manageExports,

	krillv1connect.UserServiceListUsersProcedure:       manageUsers,
	krillv1connect.UserServiceUpdateUserProcedure:      manageUsers,
	krillv1connect.UserServiceSetUserPasswordProcedure: manageUsers,
	krillv1connect.UserServiceDeleteUserProcedure:      manageUsers,
}
