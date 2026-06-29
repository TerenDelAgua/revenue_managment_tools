# 🧡 TEREN - Brand Manifesto

> **Version:** `1.0.0`  
> **Tagline:** _Flow systems. Soulful experiences._

---

## 1. What is TEREN?

**TEREN** is more than a brand—it's a philosophy of digital craftsmanship. It represents a commitment to building software that respects the user's time, intelligence, and experience.

Born from the convergence of technical excellence and human-centered design, TEREN creates products and tools that feel **effortless, elegant, and intentional**.

**Name Origin:** TEREN (derived from the creator's nickname Teren) symbolizes a fresh start, a new dawn—the "sunrise orange" that represents transformation and new beginnings.

---

## 2. Mission & Vision

### Mission

> To build digital products that eliminate friction, prioritize user flow, and prove that elegance and functionality are not mutually exclusive.

### Vision

> A world where software respects its users—where technology serves people, not the other way around. Where every interaction feels thoughtful, every feature has purpose, and complexity is hidden behind simplicity.

---

## 3. Core Values

### **Guest-First Philosophy**

We never ask for commitment before delivering value. Users should experience the product, feel its worth, and choose to engage—not be forced into registration walls.

### **Elegance Over Complexity**

If a feature adds complexity without clear user value, it doesn't belong. We choose clean architecture, minimal boilerplate, and intentional design over feature bloat.

### **Respect for Time**

Every second counts. We optimize for speed, clarity, and flow. No unnecessary loading screens, no confusing navigation, no bureaucratic hurdles.

### **Outdoor-First Design**

Our products are built for real life—used under sunlight, on the go, in motion. High contrast, legible typography, and thoughtful color palettes ensure usability in any environment.

### **Technical Integrity**

Clean code, proper architecture, and maintainability are non-negotiable. We build systems that scale gracefully and developers can understand years later.

---

## 4. Visual Identity

### **Color Palette**

| Color                    | Hex       | Usage                                         |
| ------------------------ | --------- | --------------------------------------------- |
| **Sunrise Orange**       | `#FF8C42` | Primary actions, brand accent, highlights     |
| **Orange Hover**         | `#E06B20` | Interactive states, emphasis                  |
| **Warm Stone (Bg)**      | `#F5F4F1` | Main canvas (earthy, reduces glare outdoors)  |
| **Stone Surface**        | `#FCFBFA` | Cards, elevated elements (subtle depth)       |
| **Deep Stone (Text)**    | `#1C1917` | Primary content, maximum outdoor contrast     |
| **Muted Stone (Text)**   | `#57534E` | Secondary information, high-contrast labels   |

### **Typography**

- **Primary:** Inter / Manrope (clean, geometric, highly legible)
- **Monospace:** JetBrains Mono (for technical content)
- **Scale:** Modular, base 16px, optimized for outdoor readability

### **Design Principles**

1. **Generous whitespace** → Breathing room for clarity
2. **Rounded corners (8-16px)** → Approachable, modern
3. **Subtle depth & hierarchy** → Backgrounds must nest properly (Darker canvas -> Lighter cards)
4. **High contrast for outdoors** → Usable in direct sunlight, no faint grays
5. **Fluid micro-animations** → Numbers and elements animate smoothly (e.g., `tabular-nums` and tweening)
6. **Unified Widgets over Forms** → Avoid generic SaaS form boxes. Use integrated, seamless components with subtle glowing hovers.
7. **Never block the user** → Avoid unnecessary navigation, prioritize direct interaction (Inline editing)

---

## 5. Technical Philosophy

### **The TEREN Stack**

We choose tools that align with our values:

| Layer            | Technology               | Why                                                       |
| ---------------- | ------------------------ | --------------------------------------------------------- |
| **Backend**      | Go (Golang)              | Minimalist, fast, no magic, elegant concurrency           |
| **Web**          | SvelteKit + Tailwind v4  | Zero boilerplate, reactive by default, compiler-optimized |
| **Mobile**       | Flutter                  | Native performance, single codebase, beautiful UI         |
| **Database**     | PostgreSQL               | Robust, standards-compliant, JSONB-ready                  |
| **Architecture** | Clean Architecture Light | Separation of concerns, testable, maintainable            |

### **Development Principles**

- **No framework magic** → We prefer explicit over implicit
- **Type safety** → Catch errors early, document intent
- **API-first design** → Backend and frontend evolve independently
- **Offline-first mindset** → Products work without constant connectivity
- **Privacy by default** → Zero third-party tracking, user data is sacred

---

## 6. Product Ecosystem

### **Current Products**

#### **Itinera** (Trip Planning Platform)

- **Purpose:** Plan trips without friction
- **Key Feature:** Guest-First flow—start planning immediately, register only when ready to save/share
- **Stack:** Go + SvelteKit + PostgreSQL
- **Status:** Phase 1 (MVP Development)
- **Vision:** The most respectful trip planner—no ads, no noise, just elegant organization

#### **TEREN Nihongo Flow** (Japanese Learning Tool)

- **Purpose:** Learn Japanese with focus and flow
- **Key Feature:** Offline-first SRS (Spaced Repetition System), no gamification, pure progress
- **Stack:** Flutter + Go + PostgreSQL
- **Status:** Planned (Phase 2)
- **Vision:** A tool for serious learners who value depth over engagement metrics

### **Future Products**

- **Travelers Ecosystem** (Toolkit for travelers, Ecosystem around Itinera, to make travel easier and have better experiences)
- **TEREN Design System** (Open-source UI component library)
- **Digital Nomads tools** (Improve digital Nomads travels with focus on Japan)
- **Revenue managment tools** (Toolkit for small hotels to managment reservations and revenue)

---

## 7. The Japan Connection

TEREN is deeply connected to Japan—its culture, aesthetics, and philosophy:

- **Design Inspiration:** Japanese minimalism, wabi-sabi (beauty in imperfection), ma (negative space)
- **Target Market:** International community in Japan, travelers, Japanese learners
- **Future Base:** Fukuoka, Kobe, or Kumamoto—cities that value quality of life over urban chaos
- **Cultural Alignment:** Respect, craftsmanship, continuous improvement (kaizen), humility

> _"TEREN products are designed with the spirit of 'omotenashi'—anticipating needs without intrusion, serving without demanding."_

---

## 8. Target Audience

### **Primary Users**

- **Independent travelers** who value autonomy and thoughtful design
- **Lifelong learners** (language learners, skill builders) who prefer depth over gamification
- **Digital professionals** who appreciate clean interfaces and efficient workflows
- **Japan enthusiasts** seeking tools built with cultural sensitivity

### **Secondary Audience**

- **Developers** who follow TEREN's open-source work and design philosophy
- **Startups** looking for inspiration in Guest-First product design
- **Designers** interested in outdoor-optimized, accessible interfaces

### **Thirdary Audience**

- **Small bussiness** B2C travel agencies
- **Small hotels** who needs to evolve from paper to digital.

---

## 9. Brand Voice & Tone

### **Voice Characteristics**

- **Clear, not clever** → We prioritize understanding over wit
- **Confident, not arrogant** → We know our craft, but stay humble
- **Warm, not casual** → Professional yet approachable
- **Concise, not cold** → Every word matters, but we're human

### **Tone by Context**

| Context            | Tone                          | Example                                                                 |
| ------------------ | ----------------------------- | ----------------------------------------------------------------------- |
| **Product UI**     | Direct, helpful               | "Your trip is saved locally. Create an account to sync across devices." |
| **Documentation**  | Precise, educational          | "The API returns a 201 status code on successful resource creation."    |
| **Marketing**      | Inspiring, authentic          | "Plan your adventures with elegance. No friction, just flow."           |
| **Error Messages** | Empathetic, solution-oriented | "We couldn't save your changes. Check your connection and try again."   |

---

## 10. What TEREN is NOT

❌ **Not a "hustle culture" brand** → We value sustainability over burnout  
❌ **Not chasing trends** → We choose technologies for longevity, not hype  
❌ **Not feature-heavy** → We'd rather do 3 things perfectly than 20 things adequately  
❌ **Not surveillance-based** → No tracking, no analytics that invade privacy  
❌ **Not corporate bureaucracy** → Agile, independent, responsive to users

---

## 11. The TEREN Promise

When you use a TEREN product, you can expect:

✅ **Immediate value** → No registration walls, no tutorials, just start using  
✅ **Respect for your data** → Your information is yours, encrypted, and portable  
✅ **Performance** → Fast load times, smooth interactions, offline capability  
✅ **Clarity** → No confusing menus, no hidden features, no dark patterns  
✅ **Longevity** → We build to last, with maintainable code and thoughtful updates

---

## 12. Brand Guidelines (Quick Reference)

### **Do's**

- ✅ Use sunrise orange (`#FF8C42`) as the primary accent
- ✅ Prioritize whitespace and breathing room
- ✅ Write in clear, international English
- ✅ Design for outdoor readability first
- ✅ Test on real devices in sunlight
- ✅ Keep commits clean and documented
- ✅ Respect the Guest-First flow

### **Don'ts**

- ❌ Use dark themes as default (outdoor usability)
- ❌ Add features without user validation
- ❌ Use jargon or clever copy that confuses
- ❌ Implement tracking without explicit consent
- ❌ Sacrifice performance for aesthetics
- ❌ Force registration before value delivery

---

## 13. The Founder

**Juan Carlos Del Agua Pascual (TEREN)**

- **Role:** Founder, Lead Developer, Product Designer
- **Background:** 8+ years as Senior Full Stack Developer & Tech Lead
- **Expertise:**
  - Enterprise systems (ERP, WMS, logistics)
  - Mobile development (Flutter, Kotlin, Native Android)
  - Web applications (React, Svelte, TypeScript)
  - Game development (Unreal Engine 5, Unity)
  - Team leadership & Agile methodologies
- **Personal Mission:** To create software that feels human, respects users, and proves that elegance and functionality coexist.
- **Next Chapter:** Relocating to Japan to build products at the intersection of technology, culture, and craftsmanship.

---

## 14. Connect with TEREN

- **GitHub:** `github.com/terendelagua`
- **Portfolio:** `teren.dev` (upcoming)
- **Products:** `itinera.teren.dev` (beta)
- **Contact:** `hello@teren.dev`

---

## 🌟 Final Thought

> _"TEREN is not just building apps. We're crafting experiences that honor your time, intelligence, and humanity. Every line of code, every pixel, every interaction is designed with one question in mind: 'Does this serve the user?' If the answer isn't a resounding yes, we don't build it."_

---

**Version:** 1.0.0  
**Last Updated:** April 2026  
**License:** © TEREN. All rights reserved.  
**Philosophy:** Built with intention. Designed for flow. Owned by TEREN.
