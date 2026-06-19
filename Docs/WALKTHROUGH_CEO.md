# TEREN Hotels — Reporte Ejecutivo de Proyecto

> **Fecha:** 24 de junio de 2026
> **Producto:** Suite de revenue management para hoteles pequeños en Indonesia
> **Fase actual:** MVP (meses 0–4 de 4)
> **Audiencia:** dirección, socios, cualquier persona que necesite entender dónde estamos

---

## 1. Resumen ejecutivo

**El producto funciona de extremo a extremo para las operaciones diarias de un hotel pequeño**: el personal ve el estado de cada habitación en un mapa, crea reservas, registra huéspedes, factura automáticamente y cobra. El último módulo crítico del MVP — **facturación y pagos** — está prácticamente terminado y se está cerrando esta semana.

**Estimación honesta de avance del MVP: ~80 %.** El producto ya está desplegado en Railway con una URL accesible desde fuera, por lo que cualquier persona del equipo o un tester invitado puede entrar y probar la aplicación completa sin tocar el entorno local. Lo que falta para una beta cerrada con un hotel real es, en orden: cerrar la facturación, añadir autenticación de usuarios, y configurar el hotel piloto.

**Decisión que toca tomar pronto:** si ya hay un hotel piloto comprometido, priorizamos autenticación (1–2 semanas); si no, endurecemos producto y pruebas otras 2 semanas antes de salir.

---

## 2. Qué es TEREN Hotels (en una línea)

Software de gestión para hoteles pequeños en Indonesia que reemplaza la hoja de cálculo y el papel por una herramienta web visual, rápida, en tres idiomas, y que cumple con las reglas fiscales indonesias (impuesto PPN al 11 %, numeración fiscal, auditoría, retención de datos a 10 años).

**Para quién:** hoteles boutique de 5–30 habitaciones. Hoy apunta al mercado indonesio y a la diáspora internacional (español e inglés incluidos por el equipo fundador).

**Por qué importa:** el segmento de hoteles pequeños en Indonesia sigue operando con papel, WhatsApp y Excel. No hay un competidor directo que combine simplicidad real, diseño cuidado y cumplimiento fiscal. Es una ventana de mercado estrecha pero defendible.

---

## 3. Progreso por área funcional

| Área | Estado | Qué significa en la práctica |
|---|---|---|
| Mapa interactivo de habitaciones (la "joya" del producto) | ✅ Listo | El personal ve en tiempo real qué habitaciones están libres, ocupadas, pendientes, en limpieza o bloqueadas. Ya validado en pantalla y con pruebas. |
| Reservas, check-in, check-out | ✅ Listo | Flujo completo probado. Detecta solapamientos sin bloquear al usuario. |
| Gestión de huéspedes | ✅ Listo | Base de datos con historial, reutilización silenciosa cuando vuelve un cliente conocido. |
| Facturación automática con IVA indonesio | 🟡 Casi listo (≈85 %) | Genera factura al crear reserva, calcula impuesto, permite cobros parciales, registra pagos múltiples, anula con auditoría. Faltan las pruebas de extremo a extremo. |
| Generación de PDF para el huésped | 🟡 Casi listo | Los PDFs ya se generan correctamente. Hay muestras reales en el repositorio. |
| Reportes (cierre de caja, impuesto mensual, exportable a CSV) | 🟡 En desarrollo | Los componentes existen, falta el cableado final con el backend. |
| Autenticación real (login con usuario y contraseña) | ❌ No empezado | Bloqueante para mostrar el producto a cualquier persona fuera del equipo de desarrollo. |
| Dashboard con indicadores (ocupación, ingresos del día) | ❌ No empezado | Sin ruta creada. |
| Configuración del hotel (impuestos, datos fiscales, idioma) | ❌ No empezado | Existe el lugar donde irá, pero está vacío. |
| Despliegue en servidor real (Railway) | ✅ Listo | La aplicación ya está publicada con una URL accesible desde fuera, lista para compartir con el equipo o con testers invitados. |

---

## 4. Hitos recientes

- **Diseño visual ratificado y estable.** El sistema de diseño propio de la marca (colores, tipografía, motion, dark mode) ya está formalizado y aplicado de forma consistente. Cualquier pieza nueva hereda las reglas en automático.
- **Módulo de reservas cerrado.** Se completó el ciclo check-in / check-out, asignación de habitación y registro de huéspedes, alineado con el sistema de diseño.
- **Facturación y pagos, casi cerrados.** Se construyó en orden: primero el esquema de base de datos, después la lógica de negocio, después los endpoints, después la generación de PDFs, y por último la interfaz de usuario. Cada capa tiene pruebas automatizadas. Quedan las pruebas de extremo a extremo (simular todo el flujo como lo haría un usuario real).
- **Multidioma completo.** Toda la interfaz funciona en inglés, indonesio y español. Ningún texto está "hardcodeado" en los componentes.

---

## 5. Lo que falta para hablar de beta cerrada

Lo que separa el código actual de un producto que un hotel pueda usar de verdad:

1. **Cerrar la facturación** — terminar las pruebas de extremo a extremo del flujo de cobro. Una semana con foco.
2. **Autenticación de usuarios** — login, contraseñas, separación de roles (propietario vs. recepcionista). Una semana y media.
3. **Configuración del hotel** — pantalla para que el propietario defina su impuesto, sus datos fiscales, su idioma. Tres a cinco días.
4. **Pruebas de uso real** — idealmente con un hotel piloto que pruebe el sistema durante una o dos semanas y reporte fricciones. La URL de Railway ya está lista para compartir con quien haga las pruebas.

**Plazo realista total: 2 a 3 semanas para una beta privada con un hotel real.**

---

## 6. Riesgos principales

| Riesgo | Impacto | Qué estamos haciendo al respecto |
|---|---|---|
| No hay autenticación real todavía. Cualquier persona con la URL de Railway puede ver datos en este momento. | Alto si se comparte antes de arreglarlo. | La URL ya está activa, pero no se ha compartido fuera del equipo. Es el siguiente paso crítico antes de cualquier demo externa. |
| La rama de trabajo tiene cambios pendientes de integrar. | Medio. Puede generar conflictos si se acumulan más. | Se integrarán esta semana. El contenido está cohesivo. |
| No existen todavía pruebas que simulen al usuario real de extremo a extremo. | Medio. | El plan de pruebas ya está escrito y mapeado. Falta ejecutarlo. |
| Multi-idioma, multi-hotel, datos fiscales: muchas variables. Si la primera versión falla en un edge case fiscal, la confianza del hotel piloto se rompe. | Alto en el momento de salir a producción. | El diseño fiscal está revisado contra la normativa indonesia. Se hará una pasada de validación con un contador antes del primer hotel. |
| No hay hotel piloto confirmado todavía. | Medio. | A resolver en las próximas semanas. Sin usuario real, solo tenemos hipótesis. |

---

## 7. Decisiones que necesitan dueña o dueño (no técnico)

Tres preguntas que no le corresponden resolver al equipo de desarrollo:

1. **¿Hay un hotel piloto ya comprometido, en conversaciones, o todavía no?**  
   Si la respuesta es "sí, para julio", saltamos auth + deploy al primer puesto. Si es "no", tenemos margen para endurecer el producto dos semanas más.

2. **¿Cuál es la siguiente prioridad estratégica después de facturación?**  
   Las candidatas son: autenticación (necesaria para cualquier demo externa), configuración del hotel (rápido y de alto valor), o dashboard de indicadores (más visibilidad, más "wow"). Cada una tiene un costo de tiempo distinto.

3. **¿Cuál es el modelo de cobro del producto cuando esté listo?**  
   Hoy el software es por suscripción mensual (estimación interna: 30–60 USD por hotel por mes, según tamaño). Esto define qué funcionalidades son "premium" y cuáles son la base incluida. Es una decisión de producto, no de ingeniería.

---

## 8. Plan para las próximas cuatro semanas

| Semana | Foco | Resultado esperado |
|---|---|---|
| **1** | Cerrar facturación + integrar cambios pendientes | El módulo de cobros está completo, probado y fusionado. Posibilidad de mostrar el flujo de pago funcionando en la URL de Railway. |
| **2** | Autenticación y roles | Existe login real. El propietario y el recepcionista ven cosas distintas. El producto deja de ser una demo abierta. |
| **3** | Configuración del hotel + dashboard básico | El propietario configura su hotel (impuesto, datos fiscales, idioma) y ve un resumen diario de su operación. |
| **4** | Pulido, pruebas internas, invitación a hotel piloto | Primera persona externa usando el producto de verdad, recolección de feedback real. |

**Al final del mes: beta cerrada con un hotel real usando el sistema en producción.**

---

## 9. Recursos para entender más

Si alguien quiere profundizar, estos son los documentos en orden de lectura:

- **Manifiesto de marca** — la filosofía y los valores que guían cada decisión de producto.
- **Especificación del producto** — qué módulos existen y por qué.
- **Estrategia de despliegue** — cómo se pone en un servidor cuando llegue el momento.
- **Plan de pruebas** — cómo se valida que cada cosa funciona antes de mostrarla a un usuario.

(Pedirle a Juan Carlos los enlaces directos si se necesitan.)

---

*Documento generado el 24 de junio de 2026. La foto cambia semana a semana; este es el corte de hoy.*

*Si algún número o estado no coincide con la percepción de la dirección, ajustar antes de compartir.*
