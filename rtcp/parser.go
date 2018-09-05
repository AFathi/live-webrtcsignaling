package rtcp

import (
	"errors"
)

type Parser struct {
	log ILogger
}

type Dependencies struct {
	Logger ILogger
}

func NewParser(dep Dependencies) *Parser {
	parser := new(Parser)
	parser.log = dep.Logger
	return parser
}

/*
 * parse a compound RTCP packet, return array of RTCP objects
 * FIXME: needs a big refactoring ...
 */
func (p *Parser) Parse(input IPacket) ([]interface{}, error) {
	var packets []interface{}
	var err error

	data := input.GetData()
	for {
		packet := NewPacket()
		packet.SetData(data)
		packetRTCP := NewPacketRTCP()
		err = packetRTCP.Parse(packet)
		if err != nil {
			p.log.Errorf("[RTCP]: %s", err.Error())
			return packets, err
		}
		switch packetRTCP.Header.PacketType {
		case PT_SR:
			packetSR := NewPacketSR()
			if err = packetSR.ParsePacketRTCP(packetRTCP); err != nil {
				p.log.Errorf("[RTCP]: cannot parse SR, err=%s", err.Error())
				return packets, err
			}
			p.log.Infof("%s", packetSR)
			packets = append(packets, packetSR)
		case PT_RR:
			packetRR := NewPacketRR()
			if err = packetRR.ParsePacketRTCP(packetRTCP); err != nil {
				p.log.Errorf("[RTCP]: cannot parse RR, err=%s", err.Error())
				return packets, err
			}
			p.log.Infof("%s", packetRR)
			packets = append(packets, packetRR)
		case PT_SDES:
			packetSDES := NewPacketSDES()
			if err = packetSDES.ParsePacketRTCP(packetRTCP); err != nil {
				p.log.Errorf("[RTCP]: cannot parse SDES, err=%s", err.Error())
				return packets, err
			}
			p.log.Infof("%s", packetSDES)
			packets = append(packets, packetSDES)
		case PT_BYE:
			packetBYE := NewPacketBYE()
			if err = packetBYE.ParsePacketRTCP(packetRTCP); err != nil {
				p.log.Errorf("[RTCP]: cannot parse BYE, err=%s", err.Error())
				return packets, err
			}
			p.log.Infof("%s", packetBYE)
			packets = append(packets, packetBYE)
		case PT_RTPFB:
			packetRTPFB := NewPacketRTPFB()
			if err = packetRTPFB.ParsePacketRTCP(packetRTCP); err != nil {
				p.log.Errorf("[RTCP]: packetRTPFB, err=%s", err.Error())
				return packets, err
			}
			// loading custom RTPFB packet
			switch packetRTPFB.GetMessageType() {
			case FMT_RTPFB_NACK:
				packetRTPFBNack := NewPacketRTPFBNack()
				if err = packetRTPFBNack.ParsePacketRTPFB(*packetRTPFB); err != nil {
					p.log.Errorf("[RTCP]: packetRTPFBNack, err=%s", err.Error())
					return packets, err
				}
				p.log.Infof("%s", packetRTPFBNack)
				packets = append(packets, packetRTPFBNack)
			case FMT_RTPFB_RESERVED:
				p.log.Warnf("unknown reserved packetRTPFB")
			case FMT_RTPFB_TMMBR:
				packetRTPFBTmmbr := NewPacketRTPFBTmmbr()
				if err = packetRTPFBTmmbr.ParsePacketRTPFB(*packetRTPFB); err != nil {
					p.log.Errorf("[RTCP]: packetRTPFBTmmbr, err=%s", err.Error())
					return packets, err
				}
				p.log.Infof("%s", packetRTPFBTmmbr)
				packets = append(packets, packetRTPFBTmmbr)
			case FMT_RTPFB_TMMBN:
				packetRTPFBTmmbn := NewPacketRTPFBTmmbn()
				if err = packetRTPFBTmmbn.ParsePacketRTPFB(*packetRTPFB); err != nil {
					p.log.Errorf("[RTCP]: packetRTPFBTmmbn, err=%s", err.Error())
					return packets, err
				}
				p.log.Infof("%s", packetRTPFBTmmbn)
				packets = append(packets, packetRTPFBTmmbn)
			case FMT_RTPFB_SR_REQ:
				p.log.Warnf("SR_REQ not yet implemented")
			case FMT_RTPFB_RAMS:
				p.log.Warnf("RAMS not yet implemented")
			case FMT_RTPFB_TLLEI:
				// Transport-Layer Third-Party Loss Early Indication (TLLEI)
				// @see https://tools.ietf.org/html/rfc6642#section-5.1
				// not implemented, needs a SDP rtcp-fb-nack-param = tllei
				p.log.Warnf("TLLEI not yet implemented")
			case FMT_RTPFB_ECN:
				p.log.Warnf("ECN not yet implemented")
			case FMT_RTPFB_PS:
				p.log.Warnf("PS not yet implemented")
			case FMT_RTPFB_EXT:
				p.log.Warnf("EXT not yet implemented")
			default:
				p.log.Warnf("unknown packetRTPFB")
			}
		case PT_PSFB:
			packetPSFB := NewPacketPSFB()
			if err = packetPSFB.ParsePacketRTCP(packetRTCP); err != nil {
				p.log.Errorf("[RTCP]: packetPSFB, err=%s", err.Error())
				return packets, err
			}
			switch packetPSFB.GetMessageType() {
			case FMT_PSFB_PLI:
				packetPSFBPli := NewPacketPSFBPli()
				if err = packetPSFBPli.ParsePacketPSFB(*packetPSFB); err != nil {
					p.log.Errorf("[RTCP]: packetPSFBPli, err=%s", err.Error())
					return packets, err
				}
				p.log.Infof("%s", packetPSFBPli)
				packets = append(packets, packetPSFBPli)
			case FMT_PSFB_SLI:
				packetPSFBSli := NewPacketPSFBPli()
				if err = packetPSFBSli.ParsePacketPSFB(*packetPSFB); err != nil {
					p.log.Errorf("[RTCP]: packetPSFBSli, err=%s", err.Error())
					return packets, err
				}
				p.log.Infof("%s", packetPSFBSli)
				packets = append(packets, packetPSFBSli)
			case FMT_PSFB_RPSI:
				p.log.Warnf("RPSI not yet implemented")
			case FMT_PSFB_FIR:
				packetPSFBFir := NewPacketPSFBFir()
				if err = packetPSFBFir.ParsePacketPSFB(*packetPSFB); err != nil {
					p.log.Errorf("[RTCP]: packetPSFBFir, err=%s", err.Error())
					return packets, err
				}
				p.log.Infof("%s", packetPSFBFir)
				packets = append(packets, packetPSFBFir)
			case FMT_PSFB_TSTR:
				packetPSFBTstr := NewPacketPSFBTstr()
				if err = packetPSFBTstr.ParsePacketPSFB(*packetPSFB); err != nil {
					p.log.Errorf("[RTCP]: packetPSFBTstr, err=%s", err.Error())
					return packets, err
				}
				p.log.Infof("%s", packetPSFBTstr)
				packets = append(packets, packetPSFBTstr)
			case FMT_PSFB_TSTN:
				packetPSFBTstn := NewPacketPSFBTstn()
				if err = packetPSFBTstn.ParsePacketPSFB(*packetPSFB); err != nil {
					p.log.Errorf("[RTCP]: packetPSFBTstn, err=%s", err.Error())
					return packets, err
				}
				p.log.Infof("%s", packetPSFBTstn)
				packets = append(packets, packetPSFBTstn)
			case FMT_PSFB_VBCM:
				//  H.271 Video Back Channel Message (VBCM)
				p.log.Warnf("VBCM not yet implemented")
			case FMT_PSFB_PSLEI:
				p.log.Warnf("PSLEI not yet implemented")
			case FMT_PSFB_ROI:
				p.log.Warnf("ROI not yet implemented")
			case FMT_PSFB_AFB:
				packetPSFBAfb := NewPacketPSFBAfb()
				if err = packetPSFBAfb.ParsePacketPSFB(*packetPSFB); err != nil {
					p.log.Errorf("[RTCP]: packetPSFBAfb, err=%s", err.Error())
					return packets, err
				}
				if packetPSFBAfb.IsREMB() == true {
					packetALFBRemb := NewPacketALFBRemb()
					if err = packetALFBRemb.ParsePacketPSFBAfb(*packetPSFBAfb); err != nil {
						p.log.Errorf("[RTCP]: packetALFBRemb, err=%s", err.Error())
						return packets, err
					}
					p.log.Infof("%s", packetALFBRemb)
					packets = append(packets, packetALFBRemb)
				} else {
					// default PSFBAfb packet
					p.log.Infof("%s", packetPSFBAfb)
					packets = append(packets, packetPSFBAfb)
				}
			case FMT_PSFB_EXT:
				p.log.Warnf("EXT not yet implemented")
			default:
				p.log.Warnf("unknown packetPSFB")
			}
		default:
			p.log.Warnf("[RTCP]: unhandle packet type %d", packetRTCP.Header.PacketType)
		}
		if packet.GetSize() > len(data) {
			return packets, errors.New("[RTCP] last packet overflow")
		}
		data = data[packetRTCP.GetSize():]
		if len(data) == 0 {
			break
		}
	}
	return packets, nil
}
