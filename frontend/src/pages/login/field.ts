// Field geometry for the sign-in screens, shared by the login and setup
// pages. It rides in on className rather than a `.cn-*` rule because the
// Input wrapper's own geometry is Tailwind utilities, which outrank
// layer(fluxa); twMerge settles the conflict in favour of whichever
// arrives last, which is this.
//
// Taller and rounder than the console's 36px/8px default so the fields
// match the buttons standing next to them (.cn-login-btn): 42px in one
// column, 38px once the showcase panel appears at 1100px.
//
// `md:` is restated because the wrapper ships `text-[13px] md:text-[13px]`.
// twMerge treats a variant as its own group, so an unprefixed override
// leaves the md: rule standing -- and being media-wrapped, it is emitted
// later and wins above 768px. Measured, not assumed: the field came back
// 13px until this second half was added.
export const AUTH_FIELD = "h-[42px] rounded-[12px] text-[14px] md:text-[14px] min-[1100px]:h-[38px]"
