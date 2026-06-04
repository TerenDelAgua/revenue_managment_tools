const fs = require('fs');
let content = fs.readFileSync('src/routes/bookings/+page.svelte', 'utf-8');

// Add import
if (!content.includes("import { _ } from 'svelte-i18n';")) {
    content = content.replace("import { api } from '$lib/api/client';", "import { api } from '$lib/api/client';\n\timport { _ } from 'svelte-i18n';");
}

const replaces = [
    ['Terminal de Reservas', '{$_(\'bookingsForm.title\')}'],
    ['Gestiona reservas en tiempo real, check-ins, check-outs y huéspedes.', '{$_(\'bookingsForm.subtitle\')}'],
    ['✕ Cancelar Registro', '{$_(\'bookingsForm.cancelRegistration\')}'],
    ['➕ Nueva Reserva', '{$_(\'bookingsForm.newBooking\')}'],
    ['Registrar Nueva Reserva', '{$_(\'bookingsForm.registerNewBooking\')}'],
    ['No se pudo completar el registro', '{$_(\'bookingsForm.registrationFailed\')}'],
    ['Descartar', '{$_(\'bookingsForm.dismiss\')}'],
    ['Información del Huésped', '{$_(\'bookingsForm.guestInfo\')}'],
    ['Teléfono Móvil *', '{$_(\'bookingsForm.mobilePhone\')}'],
    ['Ej: +34 612 345 678', '{$_(\'bookingsForm.phonePlaceholder\')}'],
    ['Desvincular', '{$_(\'bookingsForm.unlink\')}'],
    ['HUÉSPEDES HISTÓRICOS ENCONTRADOS', '{$_(\'bookingsForm.historicalGuests\')}'],
    ['Vincular →', '{$_(\'bookingsForm.link\')}'],
    ['Nombre Completo *', '{$_(\'bookingsForm.fullName\')}'],
    ['Nombre y Apellidos', '{$_(\'bookingsForm.fullNamePlaceholder\')}'],
    ['Email', '{$_(\'bookingsForm.email\')}'],
    ['correo@ejemplo.com', '{$_(\'bookingsForm.emailPlaceholder\')}'],
    ['Documento / Pasaporte', '{$_(\'bookingsForm.documentPassport\')}'],
    ['No. ID', '{$_(\'bookingsForm.idPlaceholder\')}'],
    ['Nacionalidad', '{$_(\'bookingsForm.nationality\')}'],
    ['ESP, IDN, etc.', '{$_(\'bookingsForm.nationalityPlaceholder\')}'],
    ['💡 Existe un perfil registrado como <strong>{detectedGuest.full_name}</strong> con este número.', '{$_(\'bookingsForm.existingProfilePrefix\')} <strong>{detectedGuest.full_name}</strong> {$_(\'bookingsForm.existingProfileSuffix\')}'],
    ['Usar Perfil', '{$_(\'bookingsForm.useProfile\')}'],
    ['Detalles de la Estancia', '{$_(\'bookingsForm.stayDetails\')}'],
    ['Check-in', '{$_(\'bookingsForm.checkIn\')}'],
    ['Check-out', '{$_(\'bookingsForm.checkOut\')}'],
    ['Adultos</label>', '{$_(\'bookingsForm.adults\')}</label>'],
    ['Niños</label>', '{$_(\'bookingsForm.children\')}</label>'],
    ['Noches</label>', '{$_(\'bookingsForm.nights\')}</label>'],
    ['{stayNights} noches', '{$_(\'bookingsForm.nightsCount\', { values: { count: stayNights } })}'],
    ['Habitación Física', '{$_(\'bookingsForm.physicalRoom\')}'],
    ['[Sin Asignar - Pendiente]', '{$_(\'bookingsForm.unassigned\')}'],
    ['Habitación {room.number}', '{$_(\'bookingsForm.roomNumber\', { values: { number: room.number } })}'],
    ['Origen del Booking', '{$_(\'bookingsForm.bookingSource\')}'],
    ['Mostrador / Walk-in', '{$_(\'bookingsForm.sources.walk_in\')}'],
    ['WhatsApp', '{$_(\'bookingsForm.sources.whatsapp\')}'],
    ['Teléfono', '{$_(\'bookingsForm.sources.phone\')}'],
    ['Booking.com', '{$_(\'bookingsForm.sources.booking_com\')}'],
    ['Airbnb', '{$_(\'bookingsForm.sources.airbnb\')}'],
    ['Agoda', '{$_(\'bookingsForm.sources.agoda\')}'],
    ['Traveloka', '{$_(\'bookingsForm.sources.traveloka\')}'],
    ['Otros Canales', '{$_(\'bookingsForm.sources.other\')}'],
    ['Importe Original ({stayNights} noches) *', '{$_(\'bookingsForm.originalAmount\', { values: { count: stayNights } })}'],
    ['Observaciones', '{$_(\'bookingsForm.observations\')}'],
    ['Peticiones especiales, intolerancias...', '{$_(\'bookingsForm.observationsPlaceholder\')}'],
    ['Forzar Reserva (Permitir sobreventa / solapamiento)', '{$_(\'bookingsForm.forceOverride\')}'],
    ['Cancelar</button>', '{$_(\'bookingsForm.cancel\')}</button>'],
    ["{formLoading ? 'Registrando...' : 'Confirmar Reserva'}", "{formLoading ? $_('bookingsForm.registering') : $_('bookingsForm.confirmBooking')}"],
    ['Próximas Llegadas', '{$_(\'bookingsForm.tabs.arrivals\')}'],
    ['Todas las Reservas', '{$_(\'bookingsForm.tabs.all\')}'],
    ['Asignaciones Pendientes', '{$_(\'bookingsForm.tabs.pending\')}'],
    ['Buscar por huésped o habitación...', '{$_(\'bookingsForm.searchPlaceholder\')}'],
    ['Mostrando <span', '{$_(\'bookingsForm.showingBookings\')} <span'],
    ['</span> reservas', '</span> {$_(\'bookingsForm.bookingsCount\')}'],
    ['No se encontraron reservas', '{$_(\'bookingsForm.noBookingsFound\')}'],
    ['Prueba cambiando el filtro o registra una reserva nueva.', '{$_(\'bookingsForm.noBookingsHint\')}'],
    ['Huésped Principal', '{$_(\'bookingsForm.columns.mainGuest\')}'],
    ['Habitación</th>', '{$_(\'bookingsForm.columns.room\')}</th>'],
    ['Fechas</th>', '{$_(\'bookingsForm.columns.dates\')}</th>'],
    ['Importe</th>', '{$_(\'bookingsForm.columns.amount\')}</th>'],
    ['Origen</th>', '{$_(\'bookingsForm.columns.source\')}</th>'],
    ['Estado</th>', '{$_(\'bookingsForm.columns.status\')}</th>'],
    ['>Sin Asignar<', '>{$_(\'bookingsForm.badges.unassigned\')}<'],
    ['>Confirmada<', '>{$_(\'bookingsForm.badges.confirmed\')}<'],
    ['>Check In<', '>{$_(\'bookingsForm.badges.checkIn\')}<'],
    ['>Check Out<', '>{$_(\'bookingsForm.badges.checkOut\')}<'],
    ['>Cancelada<', '>{$_(\'bookingsForm.badges.cancelled\')}<'],
    ['>No Show<', '>{$_(\'bookingsForm.badges.noShow\')}<']
];

for (const [search, replace] of replaces) {
    content = content.replace(search, replace);
}

const jsReplaces = [
    [/'Error al cargar reservas\.'/g, "$_('bookingsForm.toasts.loadError')"],
    [/`Huésped '\$\{guest.full_name\}' vinculado correctamente\.`/g, "$_('bookingsForm.toasts.guestLinked', { values: { name: guest.full_name } })"],
    [/'Huésped desvinculado\.'/g, "$_('bookingsForm.toasts.guestUnlinked')"],
    [/'Por favor introduce el nombre y teléfono del huésped\.'/g, "$_('bookingsForm.toasts.missingNamePhone')"],
    [/'Reserva creada! Huésped existente reutilizado automáticamente\.'/g, "$_('bookingsForm.toasts.bookingCreatedReused')"],
    [/'Reserva creada con éxito\.'/g, "$_('bookingsForm.toasts.bookingCreatedSuccess')"],
    [/'Error al registrar reserva\.'/g, "$_('bookingsForm.toasts.createError')"],
    [/'Check-in completado con éxito\.'/g, "$_('bookingsForm.toasts.checkinSuccess')"],
    [/'Error al realizar Check-in\. Asegura que la habitación esté asignada\.'/g, "$_('bookingsForm.toasts.checkinError')"],
    [/`Check-out completado\. Habitación liberada\.`/g, "$_('bookingsForm.toasts.checkoutSuccess')"],
    [/'Error al realizar Check-out\.'/g, "$_('bookingsForm.toasts.checkoutError')"],
    [/'Habitación asignada correctamente\.'/g, "$_('bookingsForm.toasts.assignSuccess')"],
    [/'Error al asignar habitación\.'/g, "$_('bookingsForm.toasts.assignError')"],
    [/'Por favor indica un motivo para la cancelación\.'/g, "$_('bookingsForm.toasts.cancelMissingReason')"],
    [/'Reserva cancelada correctamente\.'/g, "$_('bookingsForm.toasts.cancelSuccess')"],
    [/'Error al cancelar la reserva\.'/g, "$_('bookingsForm.toasts.cancelError')"],
    [/formError = 'La habitación no está disponible para las fechas seleccionadas debido a un solapamiento con otra reserva o bloqueo de inventario\.';/g, "formError = $_('bookingsForm.toasts.conflictError');"],
    [/formError = err\.message \|\| 'Error inesperado al procesar la reserva\.';/g, "formError = err.message || $_('bookingsForm.toasts.unexpectedError');"]
];

for (const [search, replace] of jsReplaces) {
    content = content.replace(search, replace);
}

fs.writeFileSync('src/routes/bookings/+page.svelte', content);
