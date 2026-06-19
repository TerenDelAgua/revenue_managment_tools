# 🎨 TEREN Design System - Component Specification

> **Version:** `1.0.0`
> **Status:** `Draft`
> **Purpose:** Documentar la base visual y el comportamiento de los componentes de la marca TEREN para estandarizar la UI en múltiples productos (Itinera, Nihongo Flow, etc.), independizándonos gradualmente de frameworks como Tailwind.

---

## 1. Principios Base

- **Claridad sobre decoración:** Cada elemento debe tener una función. Si no aporta valor o confunde, se elimina.
- **Jerarquía interactiva:** El usuario debe saber qué es clickeable, qué es información y qué es un contenedor, basándose en su comportamiento interactivo (ver sección de Movimiento).
- **Lectura en exteriores:** Alto contraste para tipografías y uso de un fondo base "Off-White" para reducir la fatiga visual bajo la luz del sol.

---

## 2. Design Tokens (Core)

_(Basado en la v1.1 estándar de diseño de TEREN)_

### 2.1 Colores

| Token             | Valor (Hex)           | Uso                                                                       |
| :---------------- | :-------------------- | :------------------------------------------------------------------------ |
| `background-base` | `#F5F4F1`             | Fondo principal de la aplicación. Blanco roto/Piedra para reducir fatiga. |
| `surface-base`    | `#FCFBFA`             | Fondo para tarjetas, modales y elementos elevados.                        |
| `primary`         | `#FF7A1A` / `#FF8C42` | Color de acento de la marca TEREN. Solo para acciones primarias.          |
| `primary-hover`   | `#E06B20`             | Estado interactivo del color primario.                                    |
| `primary-subtle`  | `#FFF7ED`             | Fondo muy suave para badges o etiquetas.                                  |
| `text-main`       | `#1C1917`             | Texto principal, títulos. Gris casi negro para máximo contraste.          |
| `text-muted`      | `#57534E`             | Texto secundario, notas. Más oscuro para garantizar legibilidad exterior. |
| `border-subtle`   | `#E7E5E4`             | Bordes en estado inactivo. Visibles pero no distractores.                 |

#### 2.1.0.1 Room Status Tokens (FMB-001)

Los tokens persistentes/derivados de cada habitación en el mapa (`RoomToken`,
`StatusLegend`). Reflejan la jerarquía de prioridad del repositorio:
`inactive > occupied > blocked > pending > cleaning > available`.

| Token              | Valor (Hex)  | Estado       | Icono | Uso                                                                    |
| :----------------- | :----------- | :----------- | :---- | :--------------------------------------------------------------------- |
| `status-available` | `#16A34A`    | `available`  | —     | Habitación limpia y vendible. Verde de "go".                           |
| `status-occupied`  | `#DC2626`    | `occupied`   | 🛏️    | Huésped en la habitación. Rojo de "no tocar".                          |
| `status-pending`   | `#D97706`    | `pending`    | ⏳    | Reserva confirmada con check-in futuro, esperando habitación.         |
| `status-blocked`   | `#44403C`    | `blocked`    | 🔧 + stripe pattern | Bloqueo explícito (maintenance / owner use / out of service). |
| `status-cleaning`  | `#0284C7`    | `cleaning`   | 🧹    | Housekeeping en curso. NO vendible. Sky-600 (frío/agua) para distinguir del verde de `available`. |
| `status-inactive`  | `#A8A29E`    | `inactive`   | —     | Habitación dada de baja (60% opacity). Máximo nivel de prioridad.     |

**Reglas de UX:**
- `cleaning` es **estado operacional transitorio**: el housekeeping lo pone al
  salir de la habitación y lo quita al terminar. Mientras está activo, la
  habitación NO se puede vender (test BT-16 en `service_test.go`).
- No usar `primary` (Sunrise Orange) para estados: el naranja es para
  *acciones del usuario*, los estados son *observación del sistema*.
- El emoji 🧹 migra a SVG `broom-stroke` en v1.1 (cuando se unifique la
  iconografía a 1.5px stroke).
- Contraste WCAG: `#0284C7` sobre `#FCFBFA` = 5.4:1 (AA), texto blanco sobre
  `#0284C7` = 4.7:1 (AA para texto grande). Si la etiqueta se renderiza con
  texto < 14px, usar `text-white` siempre.

### 2.1.1 Modo Oscuro (Black Theme v1.0)
**Filosofía:** *No invertir colores, sino transformar la luz.* Evitamos el negro absoluto (`#000000`) para prevenir fatiga visual y optimizar el consumo en pantallas OLED, manteniendo siempre el subtono cálido de la marca.

#### Paleta Dark Tokens
| Token | Valor Light | **Valor Dark** | Rationale / Uso |
|-------|-------------|----------------|-----------------|
| `background-base` | `#F5F4F1` | `#0F0E0C` | **Deep Warm Black.** Fondo principal. Cálido pero profundo. |
| `surface-base` | `#FCFBFA` | `#1C1A17` | **Warm Charcoal.** Tarjetas, modales y contenedores. |
| `primary` | `#FF8C42` | `#FF8C42` | **Identidad.** Mantiene el naranja amanecer intacto. |
| `primary-hover` | `#E06B20` | `#FFAA6B` | **Feedback.** En dark mode los hover oscuros se pierden; se aclara para visibilidad táctil. |
| `primary-subtle` | `#FFF7ED` | `#2C1F10` | **Badges/Pills.** Marrón oscuro para metadatos. |
| `text-main` | `#1C1917` | `#F5F4F1` | **Blanco Cálido.** Alto contraste WCAG AAA sobre fondos oscuros. |
| `text-muted` | `#57534E` | `#A8A29E` | **Gris Piedra.** Legible pero jerárquicamente inferior. |
| `border-subtle` | `#E7E5E4` | `#3F3D38` | **Bordes.** Define contenedores sin distracción. |
| `error-base` | `#DC2626` | `#F87171` | **Alertas.** Rojo más luminoso para detección inmediata en oscuridad. |
| `interactive-hover` | `#F5F5F4` | `#272520` | **Listas/Items.** Ligeramente más claro que la superficie para invitar a la interacción. |

#### Reglas de UI Específicas
1. **Elevación por Luz (Rim Lighting):** Las sombras (`box-shadow`) son invisibles en fondos negros. Se reemplazan por bordes superiores luminosos para simular iluminación cenital:
   ```css
   .dark .card-elevated {
     box-shadow: none;
     border-top: 1px solid rgba(255, 255, 255, 0.05);
   }

2. **Inputs y Formularios**: Los campos de texto deben ser visiblemente distintos del fondo. Usar #272520 para evitar que parezcan "agujeros" en la interfaz.
3. **Iconografía**: Prohibido usar SVGs con fill="#000000". Heredar siempre currentColor (que resuelve a text-main).
4. **Multimedia**: Aplicar brightness-95 o saturate-90 a imágenes de gran tamaño y emojis en contexto oscuro para evitar deslumbramiento en entornos con poca luz.
5. **Implementación Técnica**
   * Toggle: Clase .dark en <html> gestionada por localStorage.
   * Fallback: @media (prefers-color-scheme: dark) para usuarios sin preferencia guardada.
   * **Navegador Nativo**: html.dark { color-scheme: dark; } para forzar estilos nativos del SO (scrollbars, inputs de fecha, etc.).

### 2.2 Tipografía

- **Familia:** `Inter` (limpia, geométrica, de alta legibilidad).
- **Títulos (Headings):** Pesos `600` a `700`, con interletraje ligeramente ajustado (`tracking-tight`).
- **Cuerpo (Body):** Pesos `400` a `500`.

### 2.3 Espaciado (Spacing)

Basado en escala de 4px: `4`, `8`, `12`, `16`, `20`, `24`, `32`, `40`, `48`.

---

## 3. Especificación de Componentes

### 3.1 Botones (Buttons)

Los botones deben sentirse sólidos, accionables y directos.

- **Forma:** Bordes redondeados (`8px` o equivalente a `rounded-lg`).
- **Padding:** Espaciado cómodo, ej. `10px` vertical, `20px` horizontal.
- **Tipografía:** Peso `500` (Medium) o `600` (Semibold), centrado.
- **Comportamiento (Interacción):**
  - **Hover:** Cambia de color (ej. `primary` a `primary-hover`) y opcionalmente aumenta su sombra para denotar profundidad. **NO** se elevan en el eje Y para diferenciarse de los contenedores/tarjetas.
  - **Active (Click):** Escala a un 95% (`scale: 0.95`). Esto da la sensación táctil de "presionar".

### 3.2 Tarjetas (Cards)

Las tarjetas actúan como contenedores de agrupaciones lógicas de información.

- **Fondo:** Puro blanco (`#FFFFFF`) para destacar contra el fondo off-white de la app.
- **Forma:** Bordes más pronunciados (`12px` o `rounded-xl`).
- **Borde base:** Sutil, de `1px` color `border-subtle`.
- **Distribución de Acciones (Patrón Estándar):**
  - **Acción Principal (Añadir/Crear):** Siempre en la esquina superior derecha del header de la tarjeta (Top-Right).
  - **Navegación / Ver Todo:** Siempre en la parte inferior izquierda de la tarjeta (Bottom-Left), usualmente como un texto `text-muted` con flecha `→` que reacciona en hover.
- **Comportamiento (Interacción):**
  - Si la tarjeta es interactiva (clickeable en su totalidad), en el evento **Hover** debe:
    1. Elevarse levemente (`translate-y: -4px`).
    2. Proyectar una sombra más profunda y difuminada (opcionalmente tintada sutilmente con el color primario, ej. un tono naranja muy suave).
    3. Su borde base puede adquirir un tono semitransparente del color primario.
  - Esto diferencia inmediatamente una "Tarjeta Contenedor" de un "Botón de Acción".

### 3.3 Etiquetas y Badges (Pills)

Usados para mostrar metadatos, categorías o estados (ej. precio total de un viaje).

- **Fondo:** Color muy suave (`primary-subtle`).
- **Texto:** Color primario fuerte o un gris oscuro, peso `700` (Bold) pero en tamaño de texto pequeño (ej. `13px`).
- **Forma:** Completamente redondeada (`border-radius: 9999px` o `rounded-full`).

### 3.4 Iconos

- Deben ser simples, con trazo uniforme (usualmente de `2px` o `1.5px`).
- **Tamaño:** `16x16` para metadatos, `20x20` o `24x24` para acciones.
- **Color:** Depende del contexto. En metadatos usar `text-muted` (y opcionalmente opacidad del `70%`). En botones, toman el color del texto del botón.

### 3.5 Progressive Disclosure & Inline Forms

**Principle:** _Never block the user. Preserve context._
En lugar de modales o saltos de navegación para flujos de creación, TEREN utiliza formularios inline que se expanden dentro de la vista actual.

**Behavior Rules:**

- **Context Preservation:** El formulario aparece exactamente donde se dispara la acción. El usuario nunca pierde su posición en la lista o dashboard.
- **Motion:** `fade` o `fly` (Y: -10px → 0) con `duration: 250ms` y `easing: ease-out`. Sin overlays pesados ni backdrop blur que distraigan.
- **Focus Management:** Auto-focus en el primer input. Scroll suave al formulario si aparece fuera de viewport.
- **Optimistic UI:** Al enviar, el formulario se limpia, muestra estado de éxito sutil e inyecta el nuevo item en la lista inmediatamente. Errores async revierten con mensaje claro.
- **Field Minimization:** Solo pedir lo estrictamente necesario para crear la entidad (ej: Nombre + Fechas). Campos secundarios (Divisa, Descripción, Tags) se configuran dentro de la vista del viaje, no en la creación.

## **Why:** Los modales crean fricción cognitiva e interrumpen el flujo. Los formularios inline respetan el tiempo del usuario, mantienen la sensación "single-page" y son cruciales para usabilidad en móvil/exterior. Alineado con _"Respect for Time"_ y _"Guest-First"_.

**Inline Editing Principle:**

- Los campos editables deben ser directamente interactivos sin necesidad de un "modo edición" explícito.
- Click en texto → se convierte en input.
- Blur o Enter → guarda automáticamente.
- Sin botones "Save/Cancel" visibles hasta que sean necesarios.
- Excepción: Campos críticos que requieren confirmación explícita.

**Rationale:** Elimina fricción cognitiva, respeta el tiempo del usuario, y mantiene el contexto visual. Alineado con "Never block the user" y "Respect for Time".

### Data Display Rules

- **Never show raw keys**: Use formatted copy (`0 destinations`, not `destinos: "0"`).
- **Currency**: Standardize format globally. Use `€ 0.00` for compact views, `0.00 EUR` for tables.
- **Notes**: Display as plain text or subtle italic. No quotes, no prefixes unless ambiguous.
- **Empty States**: Hide zero-count metadata UNLESS it breaks visual rhythm or layout stability. Financial zeros (0.00 €) should be shown with reduced opacity to maintain alignment.
- **Fechas**: Usar formato relativo para cercanía ("TODAY", "TMRW") y formato exacto estándar internacional ("MMM DD, YYYY" ej. "Apr 20, 2026") para evitar ambigüedades (DD/MM vs MM/DD).
- **Date Inputs**: Hide native browser indicators. Wrap in TEREN-styled container with custom icon trigger.
- **Icons**: MVP uses emojis for speed. v1.1 migrates to unified SVGs (1.5px stroke, TEREN palette).

### Pattern: Inline Expansion vs Navigation

Para módulos secundarios que requieren espacio de edición pero no deben saturar la vista principal:

**Regla:** Si el módulo representa <20% del valor principal de la pantalla pero requiere >50% del espacio visual para su gestión completa → Usa **Drawer/Bottom Sheet** con:

- Apertura en 1 click desde un link/button sutil
- Ocupación del 60-70% de la pantalla (desktop: modal centrado max-width 600px)
- Cierre con: click fuera, botón X, o swipe down (mobile)
- Contenido scrollable independiente
- Edición inline de items (transformación de fila a formulario)
- Quick-add en la parte inferior

**Excepciones:**

- Si el módulo requiere navegación profunda (>2 niveles) → Navegación completa a ruta dedicada
- Si el módulo es crítico para el flujo principal → Tabs horizontales (solo desktop)

**Rationale:** Equilibrio entre contexto preservado y espacio de trabajo. Alineado con "Never block the user" y "Respect for Time".

### Pattern: Tap-to-Transform Lists

Para listas de datos editables (gastos, tareas, ítems):

- **Default:** Texto estático, alto contraste, sin bordes de input.
- **Interaction:** 1 tap/click transforma la fila en campos editables con focus inmediato.
- **Save/Cancel:** Enter/Blur → save. Esc/Blur sin cambios → cancel.
- **Rationale:** Elimina botones de acción redundantes, acelera el flujo, previene ediciones accidentales y mantiene la legibilidad exterior. Alineado con "Respect for Time" y "Never block the user".

### 3.6 Modales de Confirmación (Destructive Actions)

Las alertas nativas del navegador (`window.confirm`, `window.alert`) rompen la inmersión del usuario y varían drásticamente de diseño entre dispositivos. El sistema TEREN utiliza un modal personalizado para acciones destructivas (ej. eliminar un gasto, cancelar un viaje).

- **Visual:** Backdrop borroso oscuro (`bg-black/40 backdrop-blur-sm`) que aísla la decisión pero mantiene el contexto del fondo. Tarjeta modal centrada, con esquinas muy redondeadas (`rounded-2xl`).
- **Jerarquía de Color:**
  - El botón de confirmación de una acción destructiva **debe usar rojo** (ej. `bg-red-500`) en lugar del naranja principal (`primary`). Esto previene el "click ciego" por memoria muscular y advierte visualmente del peligro.
  - El botón de cancelar usa el color `text-muted` para no competir con la acción principal.
- **Movimiento:** El backdrop usa un `fade` rápido, mientras el contenedor principal usa un `fly` sutil hacia arriba (`y: 20`) o un leve `scale` con un `ease-out` natural, haciendo que la aparición sea orgánica.

### 3.7 Unified Widgets over Forms

Los formularios tradicionales (cajas individuales con bordes grises) recuerdan a los SaaS corporativos. Para crear un aspecto verdaderamente "TEREN", las entradas de datos frecuentes deben integrarse en **Widgets Unificados**.

- **Visual:** Un solo contenedor (`bg-teren-surface`) con bordes suavizados y separadores internos (`border-r` o líneas finas) en lugar de inputs separados.
- **Interacción:** El widget entero reacciona al `focus-within` o `hover` con brillos sutiles (ej. un pseudo-elemento con `bg-gradient-to-r` y `blur` para simular un halo naranja). Los `inputs` individuales carecen de bordes duros (`border-none bg-transparent focus:ring-0`).
- **Rationale:** Transmite la sensación de una herramienta de alta precisión (como un instrumento digital) en lugar de una aburrida hoja de cálculo web. Alineado con el diseño "Rugged yet Premium".

### 3.8 Selectores (Select)
- **Philosophy:** TEREN Selects are Minimalist and Native-like. We remove visual noise (arrows) and rely on context and hover states to indicate interactivity. We prioritize the "Focus Ring" for accessibility.
- **Visual Specs:**
  - **Height:** h-12 (48px) standard for touch targets.
  - **Border:** border-teren-border (subtle grey) in idle state.
  - **Background:** bg-white (clean card surface).
  - **Typography:** font-medium, centered text.
  - **Arrow:** Hidden. We use appearance-none. The hover state is enough affordance.

- **States:**
  - **Idle:** White background, grey border.
  - **Hover:** Background shifts to bg-teren-primary-subtle (very light orange). Border remains grey.
  - **Active/Focus:**
    - **Border:** Changes to border-teren-primary (solid orange).
    - **Ring:** Adds ring-2 ring-teren-primary/30 (glow effect).
  - **Implementation Detail:** Achieved via focus-within on the parent wrapper to prevent clipping.

```svelte
<!-- Wrapper Pattern -->
<div class="... focus-within:ring-2 focus-within:ring-teren-primary/30 focus-within:border-teren-primary overflow-hidden">
  <select class="appearance-none bg-transparent focus:outline-none ...">
    <!-- options -->
  </select>
</div>
```

- **Accessibility:**
  - Always include aria-label or a hidden label associated with the Select.
  - Ensure cursor-pointer is present so touch targets are clear.

### 3.9 Error States & Feedback
**Philosophy:** Errors are part of the flow, not interruptions. Communication must be immediate, contextual, and solution-oriented. Never blame the user.

#### 🎨 Visual Tokens
| Token | Valor (Hex) | Uso |
|-------|-------------|-----|
| `error-base` | `#DC2626` | Texto de error, bordes de inputs inválidos, iconos de advertencia. |
| `error-subtle` | `#FEF2F2` | Fondo para banners de error no bloqueantes. |
| `error-hover` | `#B91C1C` | Estado interactivo de botones de reintento. |

*Nota:* `#DC2626` sobre `#FCFBFA` cumple WCAG AA (contraste 4.8:1), garantizando legibilidad exterior.

#### 📐 Patrones de Comunicación
1. **Inline Field Errors (Validación en tiempo real o al enviar)**
   - Aparece directamente debajo del campo inválido.
   - Texto: `text-sm font-medium text-error-base`.
   - Input: `border-error-base` + `focus:ring-error-base/30`.
   - Layout: Reservar `min-h-[20px]` para evitar saltos visuales (layout shift).
   - Focus: Al mostrar errores, el foco se mueve automáticamente al primer campo inválido.

2. **Form/Widget Banner (Errores asíncronos o múltiples)**
   - Se muestra en la parte superior del formulario/widget.
   - Fondo: `bg-error-subtle border border-error-base/30 rounded-lg p-3`.
   - Icono: `⚠️` o `!` en círculo, tamaño `16x16`.
   - Acción: Botón "Retry" o "Dismiss" si aplica.
   - Motion: `fade` + `slide` (Y: -5px → 0) en `200ms ease-out`.

3. **Global Toasts (Errores de red, auth, sistema)**
   - Posición: Bottom-center (móvil) / Top-right (desktop).
   - Duración: 4s auto-dismiss + swipe para cerrar.
   - Contenido: Mensaje claro + botón "Reintentar" si es recuperable.
   - Fondo: `bg-teren-surface border border-teren-border shadow-lg`.
   - NO bloquea la interacción con el resto de la app.

####  Motion & Interacción
- Transiciones suaves: `200ms ease-out`. Sin pops bruscos.
- Los errores inline aparecen con `fade` sutil.
- Al corregir el campo, el mensaje de error desaparece instantáneamente (sin delay).
- Scroll suave al primer error si está fuera del viewport.

#### 🗣️ Tone of Voice Guidelines
| Contexto | Tono | Ejemplo |
|----------|------|---------|
| Validación de campos | Directo, específico | "Las fechas deben estar entre Apr 20 y Apr 30, 2026." |
| Error de red | Empático, orientado a solución | "No pudimos guardar. Verifica tu conexión o reintenta." |
| Conflicto de datos | Claro, sin jerga | "Ya existe una actividad a esa hora. Ajusta el horario o el título." |
| Genérico | Humano, responsable | "Algo salió mal. Estamos en ello. Mientras, puedes..." |

**Regla de oro:** Nunca usar "Invalid input", "Error 400", o "Failed". Siempre explicar el *qué* y el *cómo arreglarlo*.

### 3.10 Patrones de Formularios de Creación (Creation Forms)
Este patrón aplica a cualquier formulario de alta de entidades (Viajes, Actividades, Gastos) que aparezca en línea (Inline).

**Filosofía:**
*   **Contexto Primero:** El formulario se expande dentro de la lista actual (`fly` animation), nunca en un modal que oscurezca la pantalla.
*   **Minimización:** Solo pedir los campos obligatorios para existir (Nombre + Fechas). El resto se configura después.
*   **Guest-First:** Rellenar fechas por defecto (Hoy/Mañana) y asumir divisas por defecto (EUR) para acelerar el flujo.

#### 🎨 Estructura Visual (The "Unified Card")
El formulario no es una colección de inputs sueltos, sino una **Tarjeta Unificada** con jerarquía interna.

1.  **Contenedor Padre:**
    *   `bg-white`, `rounded-xl`, `border border-teren-border`, `shadow-lg`.
    *   Animación de entrada: `fly` (y: -20px → 0, duration: 250ms, ease: cubicOut).
2.  **Header:** Título claro a la izquierda, botón de cierre (`X`) a la derecha.
3.  **Cuerpo (Inputs):**
    *   **Inputs de Texto:** `bg-transparent`, `border-none`, `focus:ring-0`. Solo aparece una línea inferior sutil (`border-b`) si es necesario separar secciones, pero preferir espaciado.
    *   **Inputs de Fecha:**
        *   Ocultar indicador nativo del navegador (`appearance: none` + CSS hack para `::-webkit-calendar-picker-indicator`).
        *   Envolver en un contenedor con icono de calendario a la izquierda y borde completo (`rounded-lg`, `border`).
        *   Al hacer focus en el input interno, el contenedor padre recibe `focus-within:ring` (color primario).
4.  **Footer:**
    *   Línea divisoria superior (`border-t`).
    *   Botón "Cancelar" a la izquierda (texto `text-muted`, sin fondo).
    *   Botón "Crear" a la derecha (fondo `primary`, texto blanco).

#### 📐 Código de Estructura (Ejemplo)
```html
<!-- Contenedor Unificado -->
<div class="bg-white rounded-xl border border-teren-border shadow-lg p-6">
  
  <!-- Header -->
  <div class="flex justify-between items-center mb-6">
    <h2 class="text-xl font-bold text-teren-text-main">Create new...</h2>
    <button class="text-teren-text-muted hover:text-teren-text-main p-2">✕</button>
  </div>

  <!-- Inputs Transparentes / Unified -->
  <div class="space-y-4">
    <!-- Input Texto -->
    <input type="text" placeholder="Name..." class="w-full p-3 bg-teren-surface rounded-lg border border-teren-border focus:ring-2 focus:ring-teren-primary/30 focus:border-teren-primary outline-none transition" />
    
    <!-- Input Fecha Wrapper -->
    <div class="relative rounded-lg border border-teren-border focus-within:ring-2 focus-within:ring-teren-primary/30 focus-within:border-teren-primary transition overflow-hidden">
       <span class="absolute left-3 top-1/2 -translate-y-1/2 text-teren-text-muted">📅</span>
       <input type="date" class="w-full pl-10 pr-3 py-3 bg-transparent focus:outline-none" />
    </div>
  </div>

  <!-- Footer -->
  <div class="flex justify-end gap-3 mt-6 pt-4 border-t border-teren-border">
    <button class="text-teren-text-muted font-medium px-4 py-2 hover:bg-teren-surface rounded-lg transition">Cancel</button>
    <button class="bg-teren-primary hover:bg-teren-primary-hover text-white font-semibold px-6 py-2 rounded-lg shadow-sm active:scale-95 transition">Create</button>
  </div>

</div>
```

## 4. Animación y Movimiento (Motion)

El sistema de diseño de TEREN prioriza las **micro-animaciones** para que la interfaz se sienta viva, pero **nunca lenta**.

- **Regla General:** Todas las transiciones (cambios de color, transformaciones) deben ser de alrededor de `200ms` a `300ms` usando una curva de _easing_ natural (como `ease-out`).
- **Separación de responsabilidades de movimiento:**
  - _Elementos de Acción (Botones, Iconos de Menú):_ Reaccionan al hover cambiando atributos de color o fondo. Al interactuar (active) se encogen físicamente (`scale 0.95`).
  - _Elementos de Contenido Interactivos (Tarjetas clicables):_ Reaccionan al hover elevándose sobre el eje Z (con sombra) y el eje Y (`translate Y`). No se encogen al hacer clic de la misma manera agresiva que un botón, para preservar el peso del contenido.

### 4.1 Animación de Datos Numéricos (Dynamic Data)

Los números no deben "saltar" bruscamente. En TEREN, los valores dinámicos (como totales de dinero o distancias) se actualizan suavemente para dar una sensación de fluidez y precisión técnica.

- **Implementación (Svelte):** Usar `tweened` stores (ej. `tweened(0, { duration: 600, easing: cubicOut })`) en lugar de variables de estado directas para la visualización.
- **Estabilidad Visual:** Los números que se animan deben tener la clase tipográfica tabular (`tabular-nums`) para evitar que el contenedor tiemble horizontalmente mientras los dígitos cambian de tamaño.
