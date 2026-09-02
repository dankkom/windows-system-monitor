using Newtonsoft.Json;
using LibreHardwareMonitor.Hardware;

var computer = new Computer
{
    IsCpuEnabled = true,
    IsGpuEnabled = true,
    IsMemoryEnabled = true,
    IsMotherboardEnabled = true,
    IsControllerEnabled = true,
    IsNetworkEnabled = true,
    IsStorageEnabled = true,
};

computer.Open();
computer.Accept(new UpdateVisitor());

var sensors = new List<object>();
foreach (var hw in computer.Hardware)
{
    Collect(hw, sensors);
    foreach (var sub in hw.SubHardware)
        Collect(sub, sensors);
}

computer.Close();

Console.Write(JsonConvert.SerializeObject(sensors));

static void Collect(IHardware hw, List<object> outList)
{
    foreach (var s in hw.Sensors)
    {
        outList.Add(new
        {
            sensor_type = s.SensorType.ToString().ToLowerInvariant(),
            name = s.Name,
            label = s.Identifier.ToString(),
            value = s.Value,
            unit = UnitFor(s.SensorType),
            hardware = hw.Name,
            hw_identifier = hw.Identifier.ToString(),
        });
    }
}

static string UnitFor(SensorType t) => t switch
{
    SensorType.Temperature => "C",
    SensorType.Fan => "RPM",
    SensorType.Voltage => "V",
    SensorType.Power => "W",
    SensorType.Clock => "MHz",
    SensorType.Load => "%",
    SensorType.Current => "A",
    SensorType.Data => "GB",
    SensorType.SmallData => "MB",
    SensorType.Throughput => "B/s",
    SensorType.Frequency => "Hz",
    SensorType.TimeSpan => "s",
    SensorType.Energy => "Wh",
    SensorType.Noise => "dBA",
    _ => "",
};

class UpdateVisitor : IVisitor
{
    public void VisitComputer(IComputer c) => c.Traverse(this);
    public void VisitHardware(IHardware h)
    {
        h.Update();
        foreach (var s in h.SubHardware) s.Accept(this);
    }
    public void VisitSensor(ISensor s) { }
    public void VisitParameter(IParameter p) { }
}
